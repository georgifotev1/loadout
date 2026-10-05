package main

import (
	"net/http"

	"github.com/gfotev/loadout/internal/configurator"
)

// CheckBuild handles GET /api/builder/check.
func (app *application) CheckBuild(w http.ResponseWriter, r *http.Request, params CheckBuildParams) {
	v, err := app.configurator.Validate(r.Context(), configurator.Selection{
		CPU:         params.Cpu,
		Motherboard: params.Motherboard,
		RAM:         params.Ram,
		GPU:         params.Gpu,
		Storage:     params.Storage,
		PSU:         params.Psu,
		Case:        params.Case,
		Cooler:      params.Cooler,
	})
	if err != nil {
		app.handleError(w, r, err)
		return
	}

	resp := BuildCheck{
		Parts:           make([]BuildPart, 0, len(v.Parts)),
		Unavailable:     make([]UnavailablePart, 0, len(v.Unavailable)),
		Errors:          findings(v.Errors),
		Warnings:        findings(v.Warnings),
		Info:            findings(v.Info),
		EstimatedPowerW: v.EstimatedPowerW,
		RecommendedPsuW: v.RecommendedPSUW,
		Total:           eur(v.TotalCents),
	}
	for _, p := range v.Parts {
		resp.Parts = append(resp.Parts, BuildPart{
			Slot:    Slot(p.Slot),
			Product: summary(p.Part.ProductSummary),
			Specs:   p.Specs,
		})
	}
	for _, u := range v.Unavailable {
		resp.Unavailable = append(resp.Unavailable, UnavailablePart{Slot: Slot(u.Slot), Slug: u.Slug})
	}
	app.writeJSON(w, r, http.StatusOK, resp)
}

// ListBuilderOptions handles GET /api/builder/options.
func (app *application) ListBuilderOptions(w http.ResponseWriter, r *http.Request, params ListBuilderOptionsParams) {
	options, err := app.configurator.CompatibleOptions(r.Context(), configurator.Selection{
		CPU:         params.Cpu,
		Motherboard: params.Motherboard,
		RAM:         params.Ram,
		GPU:         params.Gpu,
		Storage:     params.Storage,
		PSU:         params.Psu,
		Case:        params.Case,
		Cooler:      params.Cooler,
	}, configurator.Slot(params.Slot))
	if err != nil {
		app.handleError(w, r, err)
		return
	}

	resp := BuilderOptions{
		Slot:    params.Slot,
		Options: make([]BuilderOption, 0, len(options)),
	}
	for _, o := range options {
		resp.Options = append(resp.Options, BuilderOption{
			Product:  summary(o.Part.ProductSummary),
			Specs:    o.Specs,
			Findings: findings(o.Findings),
		})
	}
	app.writeJSON(w, r, http.StatusOK, resp)
}

func findings(fs []configurator.Finding) []Finding {
	out := make([]Finding, 0, len(fs))
	for _, f := range fs {
		slots := make([]Slot, 0, len(f.Slots))
		for _, s := range f.Slots {
			slots = append(slots, Slot(s))
		}
		params := f.Params
		if params == nil {
			params = map[string]any{}
		}
		out = append(out, Finding{
			Rule:     FindingRule(f.Rule),
			Severity: FindingSeverity(f.Severity),
			Slots:    slots,
			Params:   params,
		})
	}
	return out
}
