package httpapi

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/go-tangra/go-tangra/v4/listquery"

	"github.com/go-tangra/go-tangra-asset/v4/internal/documents"
	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
)

// The browser list endpoints answer the list contract (go-tangra
// specs/032-server-side-tables, contracts/http-list.md): page, page_size,
// sort and order in, {items,total,page,page_size,sort,order} out. A request
// with only the old cursor / limit parameters keeps the old shape ({items})
// plus total for one release; mixing both styles is validation_failed on
// "cursor".

// legacyDefaultLimit is the page size of a legacy request without a usable
// limit (cursor only, or limit absent, invalid or < 1).
const legacyDefaultLimit = 50

// legacyLimit is the page size a legacy cursor/limit request stands for:
// always 1..listquery.MaxPageSize, legacyDefaultLimit when the limit is
// absent, invalid or < 1. A legacy request never reads an unbounded list
// (032 security review F-1).
func legacyLimit(q url.Values) int {
	n := atoiDefault(q.Get("limit"), legacyDefaultLimit)
	if n < 1 {
		n = legacyDefaultLimit
	}
	return min(n, listquery.MaxPageSize)
}

// serveList answers one list request: legacy runs the old cursor path, paged
// the list-contract page (also used, with one row, to count for legacy).
func serveList[T any](w http.ResponseWriter, r *http.Request, spec listquery.Spec,
	legacy func() ([]T, error), paged func(listquery.Request) (listquery.Page[T], error)) {
	q := r.URL.Query()
	if listquery.Legacy(q) {
		items, err := legacy()
		if err != nil {
			failList(w, err)
			return
		}
		count, err := paged(store.ListRequest(listquery.Request{PageSize: 1}, spec))
		if err != nil {
			failList(w, err)
			return
		}
		if items == nil {
			items = []T{}
		}
		WriteJSON(w, http.StatusOK, map[string]any{"items": items, "total": count.Total})
		return
	}
	req, err := listquery.Parse(q, spec)
	var le *listquery.Error
	if errors.As(err, &le) {
		WriteDetail(w, ErrValidation, map[string]any{"param": le.Param})
		return
	}
	pg, err := paged(req)
	if err != nil {
		failList(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, pg)
}

// failList maps a list error (search additionally: paperless unavailable).
func failList(w http.ResponseWriter, err error) {
	if errors.Is(err, documents.ErrSearchUnavailable) {
		WriteError(w, http.StatusServiceUnavailable, "temporarily_unavailable")
		return
	}
	failSvc(w, err)
}
