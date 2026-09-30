package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	sdk "github.com/go-tangra/go-tangra-scheduler/sdk/v4/pkg/taskexec"

	"github.com/go-tangra/go-tangra-asset/v4/internal/invsync"
)

type fakeSync struct {
	opts invsync.Options
	res  invsync.Result
	err  error
}

func (f *fakeSync) RunScheduled(_ context.Context, _ string, o invsync.Options) (invsync.Result, error) {
	f.opts = o
	return f.res, f.err
}

const tenant = "11111111-1111-7111-8111-111111111111"

func TestInventorySync(t *testing.T) {
	f := &fakeSync{res: invsync.Result{Created: 2, Updated: 1, Excluded: 3}}
	r := &Runner{Sync: f}
	h := r.Handlers()[TypeInventorySync]
	res := h(context.Background(), sdk.Request{TenantID: tenant})
	if !res.Success || !f.opts.Create || !f.opts.Update || !strings.Contains(res.Message, "3 excluded") {
		t.Fatalf("default run %+v %+v", res, f.opts)
	}
	res = h(context.Background(), sdk.Request{TenantID: tenant, Payload: json.RawMessage(`{"create":false}`)})
	if !res.Success || f.opts.Create || !f.opts.Update {
		t.Fatalf("update only %+v", f.opts)
	}
	for name, c := range map[string]struct {
		req       sdk.Request
		err       error
		permanent bool
	}{
		"platform":    {sdk.Request{}, nil, true},
		"bad payload": {sdk.Request{TenantID: tenant, Payload: json.RawMessage(`{"delete":true}`)}, nil, true},
		"unavailable": {sdk.Request{TenantID: tenant}, invsync.ErrUnavailable, false},
		"failure":     {sdk.Request{TenantID: tenant}, errors.New("db"), false},
	} {
		f.err = c.err
		res := h(context.Background(), c.req)
		if res.Success || res.Permanent != c.permanent {
			t.Errorf("%s: %+v", name, res)
		}
	}
	f.err, f.res = nil, invsync.Result{Errors: []string{"pc-1: boom"}}
	if res := h(context.Background(), sdk.Request{TenantID: tenant}); res.Success || res.Permanent {
		t.Errorf("partial failure %+v", res)
	}
	if res := (&Runner{}).InventorySync(context.Background(), sdk.Request{TenantID: tenant}); res.Success {
		t.Error("no syncer")
	}
	d := Descriptors()
	if len(d) != 1 || d[0].Type != TypeInventorySync || d[0].Platform || !json.Valid([]byte(d[0].PayloadSchema)) {
		t.Fatalf("descriptors %+v", d)
	}
}
