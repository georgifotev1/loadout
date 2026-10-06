package main

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/gfotev/loadout/internal/catalog"
)

// ListAdminProducts handles GET /api/admin/products.
func (app *application) ListAdminProducts(w http.ResponseWriter, r *http.Request, params ListAdminProductsParams) {
	page := valueOr(params.Page, 1)
	pageSize := valueOr(params.PageSize, defaultPageSize)

	res, err := app.catalog.AdminProducts(r.Context(), catalog.AdminProductFilter{
		Text:     params.Q,
		Category: string(params.Category),
		Status:   catalog.ProductStatus(params.Status),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		app.handleError(w, r, err)
		return
	}
	ids := make([]uuid.UUID, len(res.Products))
	for i, p := range res.Products {
		ids[i] = p.ID
	}
	specs, err := app.specs.SpecsByIDs(r.Context(), ids)
	if err != nil {
		app.handleError(w, r, err)
		return
	}

	resp := AdminProductList{
		Items:    make([]AdminProduct, 0, len(res.Products)),
		Page:     page,
		PageSize: pageSize,
		Total:    res.Total,
	}
	for _, p := range res.Products {
		item := AdminProduct{
			Id:          p.ID,
			Slug:        p.Slug,
			Sku:         p.SKU,
			Name:        p.Name,
			Brand:       p.Brand,
			Category:    Category(p.Category),
			Image:       p.Image,
			Active:      p.Active,
			StockQty:    p.StockQty,
			SpecsStatus: AdminProductSpecsStatusNone,
			UpdatedAt:   p.UpdatedAt,
		}
		if ps, ok := specs[p.ID]; ok {
			item.SpecsStatus = AdminProductSpecsStatus(ps.Status)
		}
		if pr := p.Price; pr != nil {
			item.Price = &AdminPrice{
				DealerCents: int64(pr.DealerCents),
				RetailCents: int64(pr.RetailCents),
				MarginCents: int64(pr.MarginCents()),
				MarginBp:    pr.MarginBP(),
			}
			if pr.RuleID != nil {
				item.Rule = &PriceRule{Id: *pr.RuleID, Scope: RuleScope(p.RuleScope)}
			}
		}
		resp.Items = append(resp.Items, item)
	}
	app.writeJSON(w, r, http.StatusOK, resp)
}
