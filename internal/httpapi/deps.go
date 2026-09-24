package httpapi

import (
	"errors"
	"net/http"

	"github.com/go-tangra/go-tangra-asset/v4/internal/assets"
	"github.com/go-tangra/go-tangra-asset/v4/internal/authz"
	"github.com/go-tangra/go-tangra-asset/v4/internal/backup"
	"github.com/go-tangra/go-tangra-asset/v4/internal/categories"
	"github.com/go-tangra/go-tangra-asset/v4/internal/consumables"
	"github.com/go-tangra/go-tangra-asset/v4/internal/documents"
	"github.com/go-tangra/go-tangra-asset/v4/internal/insurance"
	"github.com/go-tangra/go-tangra-asset/v4/internal/invsync"
	"github.com/go-tangra/go-tangra-asset/v4/internal/licenses"
	"github.com/go-tangra/go-tangra-asset/v4/internal/locations"
	"github.com/go-tangra/go-tangra-asset/v4/internal/repo"
	"github.com/go-tangra/go-tangra-asset/v4/internal/stats"
	"github.com/go-tangra/go-tangra-asset/v4/internal/stream"
	"github.com/go-tangra/go-tangra-asset/v4/internal/suppliers"
	"github.com/go-tangra/go-tangra-asset/v4/internal/userdir"
)

// Deps wire the asset HTTP handlers. Hub is optional (enables GET /stream);
// Users is optional (GET /users answers an empty list without it); Sync is
// optional (the sync routes answer 503 without an inventory client). Health
// reports the store/object-store reachability for GET /health.
type Deps struct {
	Assets      *assets.Service
	Categories  *categories.Service
	Suppliers   *suppliers.Service
	Locations   *locations.Service
	Consumables *consumables.Service
	Licenses    *licenses.Service
	Insurance   *insurance.Service
	Documents   *documents.Service
	Sync        *invsync.Service
	Stats       *stats.Service
	Backup      *backup.Service
	Users       userdir.Directory
	Hub         *stream.Hub
	Health      func() map[string]string // optional component status for /health
}

// subjects derives the authz subject from the verified platform identity. The
// gateway forwards a human/user caller on every gateway-proxied route.
func subjects(r *http.Request) (authz.Subjects, error) {
	id, err := Caller(r)
	if err != nil {
		return authz.Subjects{}, err
	}
	return authz.Subjects{TenantID: id.TenantID, UserID: id.UserID, Roles: id.Roles, ActorKind: authz.ActorUser}, nil
}

// failSvc maps a service/domain error to an HTTP response from the OpenAPI
// closed vocabulary. Not-found sentinels collapse to 404; authorization to 403;
// validation errors and a bad backup to 422; lifecycle/duplicate conflicts and
// in-use guards to 409; oversized uploads to 413; a bad media type to 415;
// inventory unavailable to 503; anything else to 500. Detail is never leaked.
func failSvc(w http.ResponseWriter, err error) {
	var (
		av assets.ValidationError
		cv categories.ValidationError
		sv suppliers.ValidationError
		lv locations.ValidationError
		kv consumables.ValidationError
		xv licenses.ValidationError
		iv insurance.ValidationError
		dv documents.ValidationError
	)
	switch {
	case errors.Is(err, assets.ErrNotFound), errors.Is(err, categories.ErrNotFound), errors.Is(err, suppliers.ErrNotFound),
		errors.Is(err, locations.ErrNotFound), errors.Is(err, consumables.ErrNotFound), errors.Is(err, licenses.ErrNotFound),
		errors.Is(err, insurance.ErrNotFound), errors.Is(err, documents.ErrNotFound), errors.Is(err, repo.ErrNotFound):
		WriteError(w, http.StatusNotFound, "not_found")
	case errors.Is(err, authz.ErrForbidden), errors.Is(err, documents.ErrCrossTenant):
		WriteError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, ErrUnauthenticated):
		WriteError(w, http.StatusUnauthorized, "unauthenticated")
	case errors.As(err, &av), errors.As(err, &cv), errors.As(err, &sv), errors.As(err, &lv), errors.As(err, &kv),
		errors.As(err, &xv), errors.As(err, &iv), errors.As(err, &dv), errors.Is(err, backup.ErrBadSchema), errors.Is(err, documents.ErrEntityType):
		WriteError(w, http.StatusUnprocessableEntity, "validation_failed")
	case errors.Is(err, assets.ErrConflict), errors.Is(err, assets.ErrState), errors.Is(err, categories.ErrConflict), errors.Is(err, categories.ErrInUse),
		errors.Is(err, suppliers.ErrConflict), errors.Is(err, suppliers.ErrInUse), errors.Is(err, locations.ErrConflict), errors.Is(err, locations.ErrInUse),
		errors.Is(err, insurance.ErrConflict), errors.Is(err, repo.ErrConflict), errors.Is(err, repo.ErrNotEmpty):
		WriteError(w, http.StatusConflict, "conflict")
	case errors.Is(err, documents.ErrTooLarge), errors.Is(err, backup.ErrTooLarge):
		WriteError(w, http.StatusRequestEntityTooLarge, "body_too_large")
	case errors.Is(err, documents.ErrMediaType):
		WriteError(w, http.StatusUnsupportedMediaType, "unsupported_media_type")
	case errors.Is(err, invsync.ErrUnavailable):
		WriteError(w, http.StatusServiceUnavailable, "temporarily_unavailable")
	default:
		WriteError(w, http.StatusInternalServerError, "internal")
	}
}
