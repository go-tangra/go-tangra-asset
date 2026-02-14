package data

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	entCrud "github.com/tx7do/go-crud/entgo"

	"github.com/go-tangra/go-tangra-asset/internal/data/ent"

	"github.com/go-tangra/go-tangra-common/middleware/audit"
)

// AuditLogRepo implements audit.AuditLogRepository for Asset service
type AuditLogRepo struct {
	entClient *entCrud.EntClient[*ent.Client]
	log       *log.Helper
}

// NewAuditLogRepo creates a new AuditLogRepo
func NewAuditLogRepo(ctx *bootstrap.Context, entClient *entCrud.EntClient[*ent.Client]) *AuditLogRepo {
	return &AuditLogRepo{
		log:       ctx.NewLoggerHelper("asset/audit_log_repo"),
		entClient: entClient,
	}
}

// CreateFromEntry implements audit.AuditLogRepository
func (r *AuditLogRepo) CreateFromEntry(ctx context.Context, entry *audit.AuditLogEntry) error {
	builder := r.entClient.Client().AuditLog.Create().
		SetAuditID(entry.AuditID).
		SetOperation(entry.Operation).
		SetServiceName(entry.ServiceName).
		SetSuccess(entry.Success).
		SetIsAuthenticated(entry.IsAuthenticated).
		SetLatencyMs(entry.LatencyMs).
		SetCreateTime(entry.Timestamp)

	if entry.TenantID > 0 {
		builder.SetTenantID(entry.TenantID)
	}
	if entry.RequestID != "" {
		builder.SetRequestID(entry.RequestID)
	}
	if entry.ClientID != "" {
		builder.SetClientID(entry.ClientID)
	}
	if entry.ClientCommonName != "" {
		builder.SetClientCommonName(entry.ClientCommonName)
	}
	if entry.ClientOrganization != "" {
		builder.SetClientOrganization(entry.ClientOrganization)
	}
	if entry.ClientSerialNumber != "" {
		builder.SetClientSerialNumber(entry.ClientSerialNumber)
	}
	if entry.ErrorCode != 0 {
		builder.SetErrorCode(entry.ErrorCode)
	}
	if entry.ErrorMessage != "" {
		builder.SetErrorMessage(entry.ErrorMessage)
	}
	if entry.PeerAddress != "" {
		builder.SetPeerAddress(entry.PeerAddress)
	}
	if entry.GeoLocation != nil {
		builder.SetGeoLocation(entry.GeoLocation)
	}
	if entry.LogHash != "" {
		builder.SetLogHash(entry.LogHash)
	}
	if entry.Signature != nil {
		builder.SetSignature(entry.Signature)
	}
	if entry.Metadata != nil {
		builder.SetMetadata(entry.Metadata)
	}

	_, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf("create audit log failed: %s", err.Error())
		return err
	}

	return nil
}
