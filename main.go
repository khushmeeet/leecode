package main

import (
	"database/sql"
	"embed"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/net/html"
)

//go:embed templates/*.html
var templateFS embed.FS

var (
	db        *sql.DB
	dbPath    string
	templates = map[string]*template.Template{}
)

func main() {
	dbPath = os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "interview.db"
	}
	var err error
	db, err = openDB(dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}

	for _, page := range []string{"dashboard", "applications", "links", "algorithms", "behavioral"} {
		templates[page] = template.Must(template.ParseFS(templateFS, "templates/layout.html", "templates/"+page+".html"))
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", handleDashboard)

	mux.HandleFunc("GET /applications", handleApplications)
	mux.HandleFunc("POST /applications/add", handleApplicationAdd)
	mux.HandleFunc("POST /applications/status", handleApplicationStatus)
	mux.HandleFunc("POST /applications/delete", handleApplicationDelete)

	mux.HandleFunc("GET /links", handleLinks)
	mux.HandleFunc("POST /links/add", handleLinkAdd)
	mux.HandleFunc("POST /links/toggle", handleLinkToggle)
	mux.HandleFunc("POST /links/delete", handleLinkDelete)

	mux.HandleFunc("GET /algorithms", handleAlgorithms)
	mux.HandleFunc("POST /algorithms/add", handleProblemAdd)
	mux.HandleFunc("POST /algorithms/delete", handleProblemDelete)

	mux.HandleFunc("GET /behavioral", handleBehavioral)
	mux.HandleFunc("POST /behavioral/add", handleStoryAdd)
	mux.HandleFunc("POST /behavioral/delete", handleStoryDelete)

	mux.HandleFunc("GET /export", handleExport)

	addr := ":" + cmp(os.Getenv("PORT"), "8080")
	log.Printf("listening on %s (db: %s)", addr, dbPath)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func cmp(s, fallback string) string {
	if s != "" {
		return s
	}
	return fallback
}

func render(w http.ResponseWriter, page string, data map[string]any) {
	if data == nil {
		data = map[string]any{}
	}
	data["Active"] = page
	if err := templates[page].ExecuteTemplate(w, "layout", data); err != nil {
		log.Printf("render %s: %v", page, err)
	}
}

func serverError(w http.ResponseWriter, err error) {
	log.Println(err)
	http.Error(w, "something went wrong", http.StatusInternalServerError)
}

// --- Dashboard ---

func handleDashboard(w http.ResponseWriter, r *http.Request) {
	counts, err := dashboardCounts(db)
	if err != nil {
		serverError(w, err)
		return
	}
	render(w, "dashboard", map[string]any{"Counts": counts})
}

// --- Applications ---

func handleApplications(w http.ResponseWriter, r *http.Request) {
	apps, err := listApplications(db)
	if err != nil {
		serverError(w, err)
		return
	}
	cols := map[string][]Application{}
	for _, a := range apps {
		cols[a.Status] = append(cols[a.Status], a)
	}
	render(w, "applications", map[string]any{
		"Applied": cols["Applied"], "Rejected": cols["Rejected"], "Accepted": cols["Accepted"],
	})
}

func handleApplicationAdd(w http.ResponseWriter, r *http.Request) {
	company := r.FormValue("company")
	if company == "" {
		http.Error(w, "company is required", http.StatusBadRequest)
		return
	}
	_, err := db.Exec(`INSERT INTO applications (company, role, url, notes) VALUES (?, ?, ?, ?)`,
		company, r.FormValue("role"), r.FormValue("url"), r.FormValue("notes"))
	if err != nil {
		serverError(w, err)
		return
	}
	http.Redirect(w, r, "/applications", http.StatusSeeOther)
}

func handleApplicationStatus(w http.ResponseWriter, r *http.Request) {
	status := r.FormValue("status")
	if status != "Applied" && status != "Rejected" && status != "Accepted" {
		http.Error(w, "bad status", http.StatusBadRequest)
		return
	}
	if _, err := db.Exec(`UPDATE applications SET status = ? WHERE id = ?`, status, r.FormValue("id")); err != nil {
		serverError(w, err)
		return
	}
	http.Redirect(w, r, "/applications", http.StatusSeeOther)
}

func handleApplicationDelete(w http.ResponseWriter, r *http.Request) {
	if _, err := db.Exec(`DELETE FROM applications WHERE id = ?`, r.FormValue("id")); err != nil {
		serverError(w, err)
		return
	}
	http.Redirect(w, r, "/applications", http.StatusSeeOther)
}

// --- Links ---

func handleLinks(w http.ResponseWriter, r *http.Request) {
	tag := r.URL.Query().Get("tag")
	links, err := listLinks(db, tag)
	if err != nil {
		serverError(w, err)
		return
	}
	tags, err := distinctValues(db, "links", "tag")
	if err != nil {
		serverError(w, err)
		return
	}
	render(w, "links", map[string]any{"Links": links, "Tags": tags, "Tag": tag})
}

func handleLinkAdd(w http.ResponseWriter, r *http.Request) {
	rawURL := strings.TrimSpace(r.FormValue("url"))
	if rawURL == "" {
		http.Error(w, "url is required", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}
	title, description := fetchMetadata(rawURL)
	if title == "" {
		// Unreachable or meta-less page: save the link anyway under its host.
		title = rawURL
		if u, err := url.Parse(rawURL); err == nil && u.Host != "" {
			title = u.Host
		}
	}
	_, err := db.Exec(`INSERT INTO links (title, url, description, tag) VALUES (?, ?, ?, ?)`,
		title, rawURL, description, r.FormValue("tag"))
	if err != nil {
		serverError(w, err)
		return
	}
	http.Redirect(w, r, "/links", http.StatusSeeOther)
}

// fetchMetadata pulls the page and returns its title and description,
// preferring Open Graph tags over the plain <title>/<meta name=description>.
// Returns empty strings on any failure; the caller decides the fallback.
func fetchMetadata(target string) (title, description string) {
	client := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequest("GET", target, nil)
	if err != nil {
		return "", ""
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; LeeCode/1.0; +personal reading list)")
	req.Header.Set("Accept", "text/html")
	resp, err := client.Do(req)
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()
	doc, err := html.Parse(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", ""
	}

	var titleTag, ogTitle, metaDesc, ogDesc string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "body":
				return // metadata lives in <head>
			case "title":
				if titleTag == "" && n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
					titleTag = n.FirstChild.Data
				}
			case "meta":
				var name, prop, content string
				for _, a := range n.Attr {
					switch a.Key {
					case "name":
						name = strings.ToLower(a.Val)
					case "property":
						prop = strings.ToLower(a.Val)
					case "content":
						content = a.Val
					}
				}
				switch {
				case prop == "og:title":
					ogTitle = content
				case prop == "og:description":
					ogDesc = content
				case name == "description":
					metaDesc = content
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	title = strings.TrimSpace(cmp(ogTitle, titleTag))
	description = strings.TrimSpace(cmp(ogDesc, metaDesc))
	if runes := []rune(description); len(runes) > 300 {
		description = string(runes[:300]) + "…"
	}
	return title, description
}

func handleLinkToggle(w http.ResponseWriter, r *http.Request) {
	if _, err := db.Exec(`UPDATE links SET read = 1 - read WHERE id = ?`, r.FormValue("id")); err != nil {
		serverError(w, err)
		return
	}
	http.Redirect(w, r, cmp(r.FormValue("back"), "/links"), http.StatusSeeOther)
}

func handleLinkDelete(w http.ResponseWriter, r *http.Request) {
	if _, err := db.Exec(`DELETE FROM links WHERE id = ?`, r.FormValue("id")); err != nil {
		serverError(w, err)
		return
	}
	http.Redirect(w, r, cmp(r.FormValue("back"), "/links"), http.StatusSeeOther)
}

// --- Algorithms ---

func handleAlgorithms(w http.ResponseWriter, r *http.Request) {
	difficulty := r.URL.Query().Get("difficulty")
	pattern := r.URL.Query().Get("pattern")
	problems, err := listProblems(db, difficulty, pattern)
	if err != nil {
		serverError(w, err)
		return
	}
	patterns, err := distinctValues(db, "problems", "pattern")
	if err != nil {
		serverError(w, err)
		return
	}
	render(w, "algorithms", map[string]any{
		"Problems": problems, "Patterns": patterns,
		"Difficulty": difficulty, "Pattern": pattern,
	})
}

func handleProblemAdd(w http.ResponseWriter, r *http.Request) {
	title := r.FormValue("title")
	if title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}
	difficulty := r.FormValue("difficulty")
	if difficulty != "Easy" && difficulty != "Medium" && difficulty != "Hard" {
		difficulty = "Easy"
	}
	_, err := db.Exec(`INSERT INTO problems (title, url, difficulty, pattern, notes) VALUES (?, ?, ?, ?, ?)`,
		title, r.FormValue("url"), difficulty, r.FormValue("pattern"), r.FormValue("notes"))
	if err != nil {
		serverError(w, err)
		return
	}
	http.Redirect(w, r, "/algorithms", http.StatusSeeOther)
}

func handleProblemDelete(w http.ResponseWriter, r *http.Request) {
	if _, err := db.Exec(`DELETE FROM problems WHERE id = ?`, r.FormValue("id")); err != nil {
		serverError(w, err)
		return
	}
	http.Redirect(w, r, cmp(r.FormValue("back"), "/algorithms"), http.StatusSeeOther)
}

// --- Behavioral ---

func handleBehavioral(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	stories, err := listStories(db, category)
	if err != nil {
		serverError(w, err)
		return
	}
	categories, err := distinctValues(db, "behavioral", "category")
	if err != nil {
		serverError(w, err)
		return
	}
	render(w, "behavioral", map[string]any{
		"Stories": stories, "Categories": categories, "Category": category,
	})
}

func handleStoryAdd(w http.ResponseWriter, r *http.Request) {
	question := r.FormValue("question")
	if question == "" {
		http.Error(w, "question is required", http.StatusBadRequest)
		return
	}
	_, err := db.Exec(`INSERT INTO behavioral (question, answer, category) VALUES (?, ?, ?)`,
		question, r.FormValue("answer"), r.FormValue("category"))
	if err != nil {
		serverError(w, err)
		return
	}
	http.Redirect(w, r, "/behavioral", http.StatusSeeOther)
}

func handleStoryDelete(w http.ResponseWriter, r *http.Request) {
	if _, err := db.Exec(`DELETE FROM behavioral WHERE id = ?`, r.FormValue("id")); err != nil {
		serverError(w, err)
		return
	}
	http.Redirect(w, r, cmp(r.FormValue("back"), "/behavioral"), http.StatusSeeOther)
}

// --- Export ---

func handleExport(w http.ResponseWriter, r *http.Request) {
	// Fold the WAL into the main db file so the download is complete on its own.
	if _, err := db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		serverError(w, fmt.Errorf("checkpoint: %w", err))
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="interview.db"`)
	http.ServeFile(w, r, dbPath)
}
