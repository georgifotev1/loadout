package main

import (
	"net/http"
	"time"

	"github.com/gfotev/loadout/internal/platform/validate"
	"github.com/gfotev/loadout/internal/specs"
)

type reviewItemJSON struct {
	ProductID   string         `json:"product_id"`
	ProductName string         `json:"product_name"`
	Category    string         `json:"category"`
	Specs       map[string]any `json:"specs"`
	Missing     []string       `json:"missing"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type reviewQueueJSON struct {
	Items    []reviewItemJSON `json:"items"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
	Total    int              `json:"total"`
}

// GET /api/admin/specs/review-queue?category=&page=&page_size=
func (app *application) specsReviewQueue(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var v validate.Validator
	page := queryInt(&v, q, "page", 1)
	pageSize := queryInt(&v, q, "page_size", defaultPageSize)
	if err := v.Err(); err != nil {
		app.handleError(w, r, err)
		return
	}

	res, err := app.specs.ReviewQueue(r.Context(), specs.ReviewFilter{
		Category: q.Get("category"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		app.handleError(w, r, err)
		return
	}

	resp := reviewQueueJSON{
		Items:    make([]reviewItemJSON, 0, len(res.Items)),
		Page:     page,
		PageSize: pageSize,
		Total:    res.Total,
	}
	for _, it := range res.Items {
		resp.Items = append(resp.Items, reviewItemJSON{
			ProductID:   it.ProductID.String(),
			ProductName: it.ProductName,
			Category:    it.Category,
			Specs:       it.Specs,
			Missing:     it.Missing,
			UpdatedAt:   it.UpdatedAt,
		})
	}
	app.writeJSON(w, r, http.StatusOK, resp)
}
