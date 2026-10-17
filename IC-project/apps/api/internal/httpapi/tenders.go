package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ic-group/ic-tender/apps/api/internal/auth"
)

// Role values as stored in the database and accepted by cmd/createuser.
const (
	roleSuperAdmin    auth.Role = "SUPER_ADMIN"
	roleJuniorAdmin   auth.Role = "JUNIOR_ADMIN"
	roleSubcontractor auth.Role = "SUBCONTRACTOR"
)

// querier is the part of *pgxpool.Pool the tenders handlers need.
type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

type tendersHandlers struct {
	db     querier
	auth   *authHandlers
	logger *slog.Logger
	now    func() time.Time
}

func (h *tendersHandlers) register(mux *http.ServeMux) {
	admins := RequireRoles(roleSuperAdmin, roleJuniorAdmin)
	everyone := RequireRoles(roleSuperAdmin, roleJuniorAdmin, roleSubcontractor)
	mux.Handle("GET /api/v1/tenders", noStore(h.auth.Authenticated(false, everyone(http.HandlerFunc(h.list)))))
	mux.Handle("GET /api/v1/tenders/{id}", noStore(h.auth.Authenticated(false, everyone(http.HandlerFunc(h.get)))))
	mux.Handle("POST /api/v1/tenders", noStore(h.auth.Authenticated(false, admins(http.HandlerFunc(h.create)))))
}

// ---------- DTOs ----------

// tenderPublicDTO is what a subcontractor may see. It deliberately has no client, no IC share,
// no initial amount and no internal comments.
type tenderPublicDTO struct {
	ID                        int64      `json:"id"`
	Code                      string     `json:"code"`
	Title                     string     `json:"title"`
	Description               *string    `json:"description"`
	Status                    string     `json:"status"`
	PublishedAt               *time.Time `json:"publishedAt"`
	DeadlineAt                time.Time  `json:"deadlineAt"`
	PlannedStartAt            *time.Time `json:"plannedStartAt"`
	PlannedEndAt              *time.Time `json:"plannedEndAt"`
	CurrentTenderAmount       string     `json:"currentTenderAmount"`
	WorksDescription          *string    `json:"worksDescription"`
	ServiceSchedule           *string    `json:"serviceSchedule"`
	RequiredCapacity          *int32     `json:"requiredCapacity"`
	SubcontractorRequirements *string    `json:"subcontractorRequirements"`
	LotsCount                 int        `json:"lotsCount"`
	Lots                      []lotDTO   `json:"lots,omitempty"`
}

// tenderAdminDTO adds the fields that only administrators may see.
type tenderAdminDTO struct {
	tenderPublicDTO
	VisibilityMode        string     `json:"visibilityMode"`
	ClientID              int64      `json:"clientId"`
	CreatedBy             int64      `json:"createdBy"`
	ApprovedBy            *int64     `json:"approvedBy"`
	ClientRevenue         *string    `json:"clientRevenue"`
	InitialICSharePercent string     `json:"initialIcSharePercent"`
	CurrentICSharePercent string     `json:"currentIcSharePercent"`
	InitialTenderAmount   string     `json:"initialTenderAmount"`
	InternalComments      *string    `json:"internalComments"`
	AwardedAt             *time.Time `json:"awardedAt"`
	CreatedAt             time.Time  `json:"createdAt"`
	UpdatedAt             time.Time  `json:"updatedAt"`
	AdminLots             []lotAdmin `json:"lots,omitempty"`
}

type lotDTO struct {
	ID                  int64   `json:"id"`
	Name                string  `json:"name"`
	Description         *string `json:"description"`
	Status              string  `json:"status"`
	CurrentTenderAmount string  `json:"currentTenderAmount"`
}

type lotAdmin struct {
	lotDTO
	InitialTenderAmount    string     `json:"initialTenderAmount"`
	AwardedSubcontractorID *int64     `json:"awardedSubcontractorId"`
	AwardedAt              *time.Time `json:"awardedAt"`
	PriceVersion           int32      `json:"priceVersion"`
}

// ---------- queries ----------

const tenderColumns = `
	t.id, t.code, t.title, t.description, t.status, t.published_at, t.deadline_at,
	t.planned_start_at, t.planned_end_at, t.current_tender_amount::text,
	t.works_description, t.service_schedule, t.required_capacity, t.subcontractor_requirements,
	(select count(*) from tender_lots l where l.tender_id = t.id),
	t.visibility_mode, t.client_id, t.created_by, t.approved_by, t.client_revenue::text,
	t.initial_ic_share_percent::text, t.current_ic_share_percent::text, t.initial_tender_amount::text,
	t.internal_comments, t.awarded_at, t.created_at, t.updated_at`

// Visibility: $1 = viewer is an administrator, $2 = viewer's subcontractor id (NULL for administrators).
// A subcontractor sees only published tenders that are open to everyone or that invited them.
const tenderVisibility = `
	($1::boolean OR (t.status = 'PUBLISHED' AND (
		t.visibility_mode = 'ALL_ELIGIBLE' OR EXISTS (
			SELECT 1 FROM tender_invited_subcontractors i
			WHERE i.tender_id = t.id AND i.subcontractor_id = $2::bigint))))`

type tenderRow struct {
	pub        tenderPublicDTO
	visibility string
	clientID   int64
	createdBy  int64
	approvedBy *int64
	revenue    *string
	initShare  string
	curShare   string
	initAmount string
	internal   *string
	awardedAt  *time.Time
	createdAt  time.Time
	updatedAt  time.Time
}

func scanTender(row pgx.Row) (tenderRow, error) {
	var t tenderRow
	err := row.Scan(
		&t.pub.ID, &t.pub.Code, &t.pub.Title, &t.pub.Description, &t.pub.Status, &t.pub.PublishedAt, &t.pub.DeadlineAt,
		&t.pub.PlannedStartAt, &t.pub.PlannedEndAt, &t.pub.CurrentTenderAmount,
		&t.pub.WorksDescription, &t.pub.ServiceSchedule, &t.pub.RequiredCapacity, &t.pub.SubcontractorRequirements,
		&t.pub.LotsCount,
		&t.visibility, &t.clientID, &t.createdBy, &t.approvedBy, &t.revenue,
		&t.initShare, &t.curShare, &t.initAmount,
		&t.internal, &t.awardedAt, &t.createdAt, &t.updatedAt,
	)
	return t, err
}

func (t tenderRow) admin() tenderAdminDTO {
	return tenderAdminDTO{
		tenderPublicDTO:       t.pub,
		VisibilityMode:        t.visibility,
		ClientID:              t.clientID,
		CreatedBy:             t.createdBy,
		ApprovedBy:            t.approvedBy,
		ClientRevenue:         t.revenue,
		InitialICSharePercent: t.initShare,
		CurrentICSharePercent: t.curShare,
		InitialTenderAmount:   t.initAmount,
		InternalComments:      t.internal,
		AwardedAt:             t.awardedAt,
		CreatedAt:             t.createdAt,
		UpdatedAt:             t.updatedAt,
	}
}

// viewer returns the arguments for tenderVisibility.
func viewer(p auth.Principal) (isAdmin bool, subID *int64) {
	if p.Role == roleSubcontractor {
		return false, p.SubcontractorID
	}
	return true, nil
}

// ---------- handlers ----------

func (h *tendersHandlers) list(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.PrincipalFrom(r.Context())
	isAdmin, subID := viewer(p)
	if !isAdmin && subID == nil {
		writeJSON(w, http.StatusOK, map[string]any{"tenders": []any{}})
		return
	}

	rows, err := h.db.Query(r.Context(),
		`SELECT`+tenderColumns+` FROM tenders t WHERE `+tenderVisibility+` ORDER BY t.deadline_at DESC, t.id DESC LIMIT 200`,
		isAdmin, subID)
	if err != nil {
		h.fail(w, "list tenders", err)
		return
	}
	defer rows.Close()

	out := make([]any, 0)
	for rows.Next() {
		t, err := scanTender(rows)
		if err != nil {
			h.fail(w, "scan tender", err)
			return
		}
		if isAdmin {
			out = append(out, t.admin())
		} else {
			out = append(out, t.pub)
		}
	}
	if err := rows.Err(); err != nil {
		h.fail(w, "iterate tenders", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tenders": out})
}

func (h *tendersHandlers) get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusNotFound, "not_found", "Тендер не найден")
		return
	}
	p, _ := auth.PrincipalFrom(r.Context())
	isAdmin, subID := viewer(p)

	t, err := scanTender(h.db.QueryRow(r.Context(),
		`SELECT`+tenderColumns+` FROM tenders t WHERE t.id = $3 AND `+tenderVisibility, isAdmin, subID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		// Same answer for "does not exist" and "not allowed": do not reveal hidden tenders.
		writeError(w, http.StatusNotFound, "not_found", "Тендер не найден")
		return
	}
	if err != nil {
		h.fail(w, "get tender", err)
		return
	}

	rows, err := h.db.Query(r.Context(),
		`SELECT id, name, description, status, current_tender_amount::text, initial_tender_amount::text,
		        awarded_subcontractor_id, awarded_at, price_version
		   FROM tender_lots WHERE tender_id = $1 ORDER BY id`, id)
	if err != nil {
		h.fail(w, "list lots", err)
		return
	}
	defer rows.Close()

	var lots []lotAdmin
	for rows.Next() {
		var l lotAdmin
		if err := rows.Scan(&l.ID, &l.Name, &l.Description, &l.Status, &l.CurrentTenderAmount, &l.InitialTenderAmount,
			&l.AwardedSubcontractorID, &l.AwardedAt, &l.PriceVersion); err != nil {
			h.fail(w, "scan lot", err)
			return
		}
		lots = append(lots, l)
	}
	if err := rows.Err(); err != nil {
		h.fail(w, "iterate lots", err)
		return
	}

	if isAdmin {
		dto := t.admin()
		dto.AdminLots = lots
		writeJSON(w, http.StatusOK, map[string]any{"tender": dto})
		return
	}
	dto := t.pub
	for _, l := range lots {
		dto.Lots = append(dto.Lots, l.lotDTO)
	}
	writeJSON(w, http.StatusOK, map[string]any{"tender": dto})
}

var moneyRe = regexp.MustCompile(`^\d{1,12}(\.\d{1,2})?$`)

type createLot struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Amount      string  `json:"amount"`
}

type createTenderInput struct {
	Code                      string      `json:"code"`
	Title                     string      `json:"title"`
	Description               *string     `json:"description"`
	ClientID                  int64       `json:"clientId"`
	DeadlineAt                time.Time   `json:"deadlineAt"`
	PlannedStartAt            *time.Time  `json:"plannedStartAt"`
	PlannedEndAt              *time.Time  `json:"plannedEndAt"`
	ClientRevenue             *string     `json:"clientRevenue"`
	InitialTenderAmount       string      `json:"initialTenderAmount"`
	VisibilityMode            string      `json:"visibilityMode"`
	WorksDescription          *string     `json:"worksDescription"`
	ServiceSchedule           *string     `json:"serviceSchedule"`
	RequiredCapacity          *int32      `json:"requiredCapacity"`
	SubcontractorRequirements *string     `json:"subcontractorRequirements"`
	InternalComments          *string     `json:"internalComments"`
	Lots                      []createLot `json:"lots"`
}

func (in *createTenderInput) validate(now time.Time) string {
	in.Code = strings.TrimSpace(in.Code)
	in.Title = strings.TrimSpace(in.Title)
	switch {
	case in.Code == "" || len(in.Code) > 64:
		return "Код тендера обязателен (до 64 символов)"
	case in.Title == "" || len(in.Title) > 300:
		return "Название тендера обязательно (до 300 символов)"
	case in.ClientID <= 0:
		return "Укажите клиента (clientId)"
	case in.DeadlineAt.IsZero() || !in.DeadlineAt.After(now):
		return "Дедлайн должен быть в будущем"
	case !moneyRe.MatchString(in.InitialTenderAmount):
		return "Сумма тендера указывается числом, например 1500000.00"
	case in.ClientRevenue != nil && !moneyRe.MatchString(*in.ClientRevenue):
		return "Выручка клиента указывается числом, например 2000000.00"
	case in.PlannedStartAt != nil && in.PlannedEndAt != nil && !in.PlannedEndAt.After(*in.PlannedStartAt):
		return "Окончание работ должно быть позже начала"
	case in.RequiredCapacity != nil && *in.RequiredCapacity <= 0:
		return "Необходимая мощность должна быть больше нуля"
	case len(in.Lots) > 50:
		return "Не более 50 лотов"
	}
	if in.VisibilityMode == "" {
		in.VisibilityMode = "ALL_ELIGIBLE"
	}
	if in.VisibilityMode != "ALL_ELIGIBLE" && in.VisibilityMode != "INVITED_ONLY" {
		return "visibilityMode: ALL_ELIGIBLE или INVITED_ONLY"
	}
	for i := range in.Lots {
		in.Lots[i].Name = strings.TrimSpace(in.Lots[i].Name)
		if in.Lots[i].Name == "" {
			return "У каждого лота должно быть название"
		}
		if !moneyRe.MatchString(in.Lots[i].Amount) {
			return "Сумма лота указывается числом, например 500000.00"
		}
	}
	return ""
}

// create stores a new tender as a DRAFT. Publishing and approval are separate steps that are not built yet.
// Both amounts start equal; the pricing service is expected to change current_* later.
func (h *tendersHandlers) create(w http.ResponseWriter, r *http.Request) {
	var in createTenderInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if msg := in.validate(h.now()); msg != "" {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", msg)
		return
	}
	p, _ := auth.PrincipalFrom(r.Context())

	tx, err := h.db.Begin(r.Context())
	if err != nil {
		h.fail(w, "begin", err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()

	var id int64
	err = tx.QueryRow(r.Context(),
		`INSERT INTO tenders (code, title, description, status, client_id, created_by, deadline_at,
		                      planned_start_at, planned_end_at, client_revenue, initial_tender_amount, current_tender_amount,
		                      visibility_mode, works_description, service_schedule, required_capacity,
		                      subcontractor_requirements, internal_comments)
		 VALUES ($1, $2, $3, 'DRAFT', $4, $5, $6, $7, $8, $9::numeric, $10::numeric, $10::numeric, $11, $12, $13, $14, $15, $16)
		 RETURNING id`,
		in.Code, in.Title, in.Description, in.ClientID, p.UserID, in.DeadlineAt,
		in.PlannedStartAt, in.PlannedEndAt, in.ClientRevenue, in.InitialTenderAmount,
		in.VisibilityMode, in.WorksDescription, in.ServiceSchedule, in.RequiredCapacity,
		in.SubcontractorRequirements, in.InternalComments,
	).Scan(&id)
	if err != nil {
		h.createError(w, err)
		return
	}
	for _, l := range in.Lots {
		if _, err := tx.Exec(r.Context(),
			`INSERT INTO tender_lots (tender_id, name, description, initial_tender_amount, current_tender_amount)
			 VALUES ($1, $2, $3, $4::numeric, $4::numeric)`, id, l.Name, l.Description, l.Amount); err != nil {
			h.createError(w, err)
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		h.fail(w, "commit tender", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (h *tendersHandlers) createError(w http.ResponseWriter, err error) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			writeError(w, http.StatusConflict, "duplicate_code", "Тендер с таким кодом уже существует")
			return
		case "23503":
			writeError(w, http.StatusUnprocessableEntity, "unknown_client", "Клиент с таким id не найден")
			return
		case "23514", "22P02", "22003":
			writeError(w, http.StatusUnprocessableEntity, "validation_failed", "Данные не прошли проверку")
			return
		}
	}
	h.fail(w, "create tender", err)
}

func (h *tendersHandlers) fail(w http.ResponseWriter, what string, err error) {
	h.logger.Error("tenders request failed", "op", what, "error", err)
	writeError(w, http.StatusInternalServerError, "internal_error", "Внутренняя ошибка сервера")
}
