package main

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/gfotev/loadout/internal/specs"
)

// ListSpecsReviewQueue handles GET /api/admin/specs/review-queue.
func (app *application) ListSpecsReviewQueue(w http.ResponseWriter, r *http.Request, params ListSpecsReviewQueueParams) {
	page := valueOr(params.Page, 1)
	pageSize := valueOr(params.PageSize, defaultPageSize)

	res, err := app.specs.ReviewQueue(r.Context(), specs.ReviewFilter{
		Category: string(params.Category),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		app.handleError(w, r, err)
		return
	}

	resp := ReviewQueue{
		Items:    make([]ReviewItem, 0, len(res.Items)),
		Page:     page,
		PageSize: pageSize,
		Total:    res.Total,
	}
	for _, it := range res.Items {
		resp.Items = append(resp.Items, ReviewItem{
			ProductId:   it.ProductID,
			ProductName: it.ProductName,
			Category:    Category(it.Category),
			Missing:     it.Missing,
			UpdatedAt:   it.UpdatedAt,
		})
	}
	app.writeJSON(w, r, http.StatusOK, resp)
}

// GetProductSpecs handles GET /api/admin/specs/{product_id}.
func (app *application) GetProductSpecs(w http.ResponseWriter, r *http.Request, productID uuid.UUID) {
	resp, err := app.productSpecs(r.Context(), productID)
	if err != nil {
		app.handleError(w, r, err)
		return
	}
	app.writeJSON(w, r, http.StatusOK, resp)
}

// UpdateProductSpecs handles PATCH /api/admin/specs/{product_id}.
func (app *application) UpdateProductSpecs(w http.ResponseWriter, r *http.Request, productID uuid.UUID) {
	var in SpecsUpdate
	if err := readJSON(w, r, &in); err != nil {
		app.handleError(w, r, err)
		return
	}
	values := make(map[string]any, len(in.Values))
	for field, v := range in.Values {
		if v == nil {
			values[field] = nil // JSON null: clear the admin value
		} else {
			values[field] = *v
		}
	}
	if _, err := app.specs.SetFields(r.Context(), productID, values); err != nil {
		app.handleError(w, r, err)
		return
	}

	resp, err := app.productSpecs(r.Context(), productID)
	if err != nil {
		app.handleError(w, r, err)
		return
	}
	app.writeJSON(w, r, http.StatusOK, resp)
}

// productSpecs joins a product's spec fields with the catalog's product, whose attributes the
// admin reads values from.
func (app *application) productSpecs(ctx context.Context, productID uuid.UUID) (ProductSpecs, error) {
	pf, err := app.specs.Fields(ctx, productID)
	if err != nil {
		return ProductSpecs{}, err
	}
	p, err := app.catalog.ProductByID(ctx, productID)
	if err != nil {
		return ProductSpecs{}, err
	}

	resp := ProductSpecs{
		ProductId:   p.ID,
		ProductName: p.Name,
		Slug:        p.Slug,
		Sku:         p.ProviderSKU,
		Brand:       p.Brand,
		Category:    Category(pf.Category),
		Active:      p.Active,
		Attributes:  p.Attributes,
		Status:      SpecStatus(pf.Status),
		Missing:     pf.Missing,
		Fields:      make([]SpecField, 0, len(pf.Fields)),
		UpdatedAt:   pf.UpdatedAt,
	}
	if len(p.Images) > 0 {
		resp.Image = p.Images[0]
	}
	if resp.Attributes == nil {
		resp.Attributes = map[string]string{}
	}
	if resp.Missing == nil {
		resp.Missing = []string{}
	}
	for _, f := range pf.Fields {
		resp.Fields = append(resp.Fields, SpecField{
			Name:     f.Name,
			Kind:     SpecKind(f.Kind),
			Required: f.Required,
			Supplier: f.Supplier,
			Admin:    f.Admin,
			Value:    f.Value,
		})
	}
	return resp, nil
}
