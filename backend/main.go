package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"regexp"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schema string

var (
	phoneRe  = regexp.MustCompile(`^[6-9]\d{9}$`) // Indian mobile, 10 digits, no +91
	stages   = []string{"NEW", "CONTACTED", "MEETING", "PROPOSAL", "WON", "LOST"}
	products = []string{"TERM", "ENDOWMENT", "ULIP", "MONEY_BACK", "PENSION", "HEALTH"}
	freqs    = []string{"MONTHLY", "QUARTERLY", "HALF_YEARLY", "YEARLY"}
)

type Lead struct {
	ID            int64  `json:"id" db:"id"`
	Name          string `json:"name" db:"name"`
	Phone         string `json:"phone" db:"phone"`
	Email         string `json:"email" db:"email"`
	City          string `json:"city" db:"city"`
	State         string `json:"state" db:"state"`
	Product       string `json:"product" db:"product"`
	Stage         string `json:"stage" db:"stage"`
	PremiumBudget int64  `json:"premiumBudget" db:"premium_budget"`
	Notes         string `json:"notes" db:"notes"`
	CreatedAt     string `json:"createdAt" db:"created_at"`
}

type Policy struct {
	ID            int64  `json:"id" db:"id"`
	LeadID        int64  `json:"leadId" db:"lead_id"`
	LeadName      string `json:"leadName" db:"lead_name"`
	Insurer       string `json:"insurer" db:"insurer"`
	PolicyNumber  string `json:"policyNumber" db:"policy_number"`
	Product       string `json:"product" db:"product"`
	SumAssured    int64  `json:"sumAssured" db:"sum_assured"`
	AnnualPremium int64  `json:"annualPremium" db:"annual_premium"`
	Frequency     string `json:"frequency" db:"frequency"`
	NextDueDate   string `json:"nextDueDate" db:"next_due_date"`
}

type Followup struct {
	ID       int64  `json:"id" db:"id"`
	LeadID   int64  `json:"leadId" db:"lead_id"`
	LeadName string `json:"leadName" db:"lead_name"`
	DueDate  string `json:"dueDate" db:"due_date"`
	Note     string `json:"note" db:"note"`
	Done     bool   `json:"done" db:"done"`
}

type App struct{ db *pgxpool.Pool }

func main() {
	ctx := context.Background()
	var pool *pgxpool.Pool
	var err error
	for i := 0; i < 20; i++ { // wait for DB
		if pool, err = pgxpool.New(ctx, os.Getenv("DATABASE_URL")); err == nil {
			if err = pool.Ping(ctx); err == nil {
				break
			}
		}
		time.Sleep(time.Second)
	}
	if err != nil {
		log.Fatal(err)
	}
	if _, err := pool.Exec(ctx, schema); err != nil {
		log.Fatal(err)
	}
	a := &App{pool}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /api/v1/dashboard", a.dashboard)
	mux.HandleFunc("GET /api/v1/leads", a.listLeads)
	mux.HandleFunc("POST /api/v1/leads", a.saveLead)
	mux.HandleFunc("PUT /api/v1/leads/{id}", a.saveLead)
	mux.HandleFunc("DELETE /api/v1/leads/{id}", a.deleteByID("leads"))
	mux.HandleFunc("GET /api/v1/policies", a.listPolicies)
	mux.HandleFunc("POST /api/v1/policies", a.createPolicy)
	mux.HandleFunc("DELETE /api/v1/policies/{id}", a.deleteByID("policies"))
	mux.HandleFunc("GET /api/v1/followups", a.listFollowups)
	mux.HandleFunc("POST /api/v1/followups", a.createFollowup)
	mux.HandleFunc("PATCH /api/v1/followups/{id}/done", a.markDone)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Println("listening on :" + port)
	log.Fatal(http.ListenAndServe(":"+port, cors(mux)))
}

func cors(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(204)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, code int, msg string) { writeJSON(w, code, map[string]string{"error": msg}) }
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(v); err != nil {
		fail(w, 400, "invalid JSON body")
		return false
	}
	return true
}
func list[T any](w http.ResponseWriter, r *http.Request, db *pgxpool.Pool, q string, args ...any) {
	rows, err := db.Query(r.Context(), q, args...)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	out, err := pgx.CollectRows(rows, pgx.RowToStructByName[T])
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	if out == nil {
		out = []T{}
	}
	writeJSON(w, 200, out)
}

func (a *App) deleteByID(table string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tag, err := a.db.Exec(r.Context(), "DELETE FROM "+table+" WHERE id=$1", r.PathValue("id"))
		if err != nil || tag.RowsAffected() == 0 {
			fail(w, 404, "not found")
			return
		}
		w.WriteHeader(204)
	}
}

const leadCols = `id,name,phone,email,city,state,product,stage,premium_budget,notes,created_at::text AS created_at`

func (a *App) listLeads(w http.ResponseWriter, r *http.Request) {
	list[Lead](w, r, a.db, `SELECT `+leadCols+` FROM leads
		WHERE ($1='' OR stage=$1) AND ($2='' OR name ILIKE '%'||$2||'%' OR phone LIKE '%'||$2||'%' OR city ILIKE '%'||$2||'%')
		ORDER BY id DESC`, r.URL.Query().Get("stage"), r.URL.Query().Get("q"))
}

func (a *App) saveLead(w http.ResponseWriter, r *http.Request) {
	var l Lead
	if !decode(w, r, &l) {
		return
	}
	if l.Product == "" {
		l.Product = "TERM"
	}
	if l.Stage == "" {
		l.Stage = "NEW"
	}
	switch {
	case l.Name == "":
		fail(w, 400, "name is required")
	case !phoneRe.MatchString(l.Phone):
		fail(w, 400, "phone must be a 10-digit Indian mobile number")
	case !slices.Contains(stages, l.Stage):
		fail(w, 400, "invalid stage")
	case !slices.Contains(products, l.Product):
		fail(w, 400, "invalid product")
	case l.PremiumBudget < 0:
		fail(w, 400, "premiumBudget cannot be negative")
	default:
		var row pgx.Row
		if id := r.PathValue("id"); id != "" {
			row = a.db.QueryRow(r.Context(), `UPDATE leads SET name=$1,phone=$2,email=$3,city=$4,state=$5,product=$6,stage=$7,premium_budget=$8,notes=$9 WHERE id=$10 RETURNING id,created_at::text`,
				l.Name, l.Phone, l.Email, l.City, l.State, l.Product, l.Stage, l.PremiumBudget, l.Notes, id)
		} else {
			row = a.db.QueryRow(r.Context(), `INSERT INTO leads(name,phone,email,city,state,product,stage,premium_budget,notes) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id,created_at::text`,
				l.Name, l.Phone, l.Email, l.City, l.State, l.Product, l.Stage, l.PremiumBudget, l.Notes)
		}
		if err := row.Scan(&l.ID, &l.CreatedAt); err != nil {
			fail(w, 404, "lead not found")
			return
		}
		code := 201
		if r.Method == http.MethodPut {
			code = 200
		}
		writeJSON(w, code, l)
	}
}

func (a *App) listPolicies(w http.ResponseWriter, r *http.Request) {
	list[Policy](w, r, a.db, `SELECT p.id,p.lead_id,l.name AS lead_name,p.insurer,p.policy_number,p.product,p.sum_assured,p.annual_premium,p.frequency,p.next_due_date::text AS next_due_date
		FROM policies p JOIN leads l ON l.id=p.lead_id ORDER BY p.next_due_date`)
}

func (a *App) createPolicy(w http.ResponseWriter, r *http.Request) {
	var p Policy
	if !decode(w, r, &p) {
		return
	}
	if p.Product == "" {
		p.Product = "TERM"
	}
	if p.Frequency == "" {
		p.Frequency = "YEARLY"
	}
	if p.LeadID == 0 || p.Insurer == "" || p.PolicyNumber == "" || p.NextDueDate == "" ||
		!slices.Contains(products, p.Product) || !slices.Contains(freqs, p.Frequency) || p.SumAssured < 0 || p.AnnualPremium < 0 {
		fail(w, 400, "leadId, insurer, policyNumber, nextDueDate and valid product/frequency are required")
		return
	}
	err := a.db.QueryRow(r.Context(), `INSERT INTO policies(lead_id,insurer,policy_number,product,sum_assured,annual_premium,frequency,next_due_date)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`, p.LeadID, p.Insurer, p.PolicyNumber, p.Product, p.SumAssured, p.AnnualPremium, p.Frequency, p.NextDueDate).Scan(&p.ID)
	if err != nil {
		fail(w, 400, "could not save policy (check leadId and date format YYYY-MM-DD)")
		return
	}
	writeJSON(w, 201, p)
}

func (a *App) listFollowups(w http.ResponseWriter, r *http.Request) {
	list[Followup](w, r, a.db, `SELECT f.id,f.lead_id,l.name AS lead_name,f.due_date::text AS due_date,f.note,f.done
		FROM followups f JOIN leads l ON l.id=f.lead_id ORDER BY f.done, f.due_date`)
}

func (a *App) createFollowup(w http.ResponseWriter, r *http.Request) {
	var f Followup
	if !decode(w, r, &f) {
		return
	}
	if f.LeadID == 0 || f.DueDate == "" {
		fail(w, 400, "leadId and dueDate are required")
		return
	}
	if err := a.db.QueryRow(r.Context(), `INSERT INTO followups(lead_id,due_date,note) VALUES($1,$2,$3) RETURNING id`, f.LeadID, f.DueDate, f.Note).Scan(&f.ID); err != nil {
		fail(w, 400, "could not save follow-up (check leadId and date format YYYY-MM-DD)")
		return
	}
	writeJSON(w, 201, f)
}

func (a *App) markDone(w http.ResponseWriter, r *http.Request) {
	tag, err := a.db.Exec(r.Context(), `UPDATE followups SET done=true WHERE id=$1`, r.PathValue("id"))
	if err != nil || tag.RowsAffected() == 0 {
		fail(w, 404, "not found")
		return
	}
	w.WriteHeader(204)
}

func (a *App) dashboard(w http.ResponseWriter, r *http.Request) {
	out := map[string]any{}
	stageCounts := map[string]int{}
	rows, err := a.db.Query(r.Context(), `SELECT stage, count(*) FROM leads GROUP BY stage`)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	for rows.Next() {
		var s string
		var c int
		rows.Scan(&s, &c)
		stageCounts[s] = c
	}
	rows.Close()
	var dueCount, overdue int
	var dueAmount, book int64
	a.db.QueryRow(r.Context(), `SELECT count(*), COALESCE(sum(annual_premium),0) FROM policies WHERE next_due_date BETWEEN current_date AND current_date+30`).Scan(&dueCount, &dueAmount)
	a.db.QueryRow(r.Context(), `SELECT COALESCE(sum(annual_premium),0) FROM policies`).Scan(&book)
	a.db.QueryRow(r.Context(), `SELECT count(*) FROM followups WHERE NOT done AND due_date<=current_date`).Scan(&overdue)
	out["stageCounts"] = stageCounts
	out["renewalsDue30d"] = dueCount
	out["renewalPremiumDue30d"] = dueAmount
	out["annualPremiumBook"] = book
	out["followupsDueOrOverdue"] = overdue
	writeJSON(w, 200, out)
}
