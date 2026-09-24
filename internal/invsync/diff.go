package invsync

import (
	"sort"
	"strings"

	"github.com/go-tangra/go-tangra-asset/v4/internal/invclient"
	"github.com/go-tangra/go-tangra-asset/v4/internal/store"
)

// Change actions.
const (
	ActionCreate    = "create"
	ActionUpdate    = "update"
	ActionUnchanged = "unchanged"
)

// Tag keys the sync writes on an asset.
const (
	TagInventoryHostID = "inventory_host_id"
	TagManufacturer    = "manufacturer"
	TagOSName          = "os_name"
	TagOSVersion       = "os_version"
	TagOSArch          = "os_arch"
)

// FieldChange is one differing field.
type FieldChange struct {
	Old string `json:"old"`
	New string `json:"new"`
}

// Change is the sync decision for one inventory host.
type Change struct {
	HostID   string                 `json:"host_id"`
	Hostname string                 `json:"hostname"`
	Serial   string                 `json:"serial,omitempty"`
	Action   string                 `json:"action"`
	AssetID  string                 `json:"asset_id,omitempty"`
	AssetTag string                 `json:"asset_tag,omitempty"`
	Changes  map[string]FieldChange `json:"changes,omitempty"`
}

// Desired is the asset projection of an inventory host.
type Desired struct {
	Name, Serial, ModelName string
	Tags                    map[string]string
}

// desired projects a host onto the asset fields the sync owns.
func desired(h invclient.Host) Desired {
	name := strings.TrimSpace(h.Hostname)
	if name == "" {
		name = strings.TrimSpace(h.SystemSerial)
	}
	if name == "" {
		name = h.ID
	}
	tags := map[string]string{}
	if h.ID != "" {
		tags[TagInventoryHostID] = h.ID
	}
	if v := strings.TrimSpace(h.Manufacturer); v != "" {
		tags[TagManufacturer] = v
	}
	if v := strings.TrimSpace(h.OSName); v != "" {
		tags[TagOSName] = v
	}
	if v := strings.TrimSpace(h.OSVersion); v != "" {
		tags[TagOSVersion] = v
	}
	if v := strings.TrimSpace(h.OSArch); v != "" {
		tags[TagOSArch] = v
	}
	return Desired{Name: name, Serial: strings.TrimSpace(h.SystemSerial), ModelName: strings.TrimSpace(h.Model), Tags: tags}
}

// index builds lookup tables over the tenant's assets: by serial (case-
// insensitive), by inventory host id tag, and by name (hostname match).
type index struct {
	bySerial map[string]store.Asset
	byHostID map[string]store.Asset
	byName   map[string]store.Asset
}

func buildIndex(assets []store.Asset) index {
	ix := index{bySerial: map[string]store.Asset{}, byHostID: map[string]store.Asset{}, byName: map[string]store.Asset{}}
	for _, a := range assets {
		if s := strings.ToLower(strings.TrimSpace(a.Serial)); s != "" {
			if _, dup := ix.bySerial[s]; !dup {
				ix.bySerial[s] = a
			}
		}
		if id := a.Tags[TagInventoryHostID]; id != "" {
			if _, dup := ix.byHostID[id]; !dup {
				ix.byHostID[id] = a
			}
		}
		if n := strings.ToLower(strings.TrimSpace(a.Name)); n != "" {
			if _, dup := ix.byName[n]; !dup {
				ix.byName[n] = a
			}
		}
	}
	return ix
}

// match finds the asset a host corresponds to: inventory host id, then serial,
// then hostname.
func (ix index) match(h invclient.Host) (store.Asset, bool) {
	if h.ID != "" {
		if a, ok := ix.byHostID[h.ID]; ok {
			return a, true
		}
	}
	if s := strings.ToLower(strings.TrimSpace(h.SystemSerial)); s != "" {
		if a, ok := ix.bySerial[s]; ok {
			return a, true
		}
	}
	if n := strings.ToLower(strings.TrimSpace(h.Hostname)); n != "" {
		if a, ok := ix.byName[n]; ok {
			return a, true
		}
	}
	return store.Asset{}, false
}

// diffOne compares a matched asset with the desired projection.
func diffOne(a store.Asset, d Desired) map[string]FieldChange {
	out := map[string]FieldChange{}
	if a.Name != d.Name {
		out["name"] = FieldChange{a.Name, d.Name}
	}
	if a.Serial != d.Serial && d.Serial != "" {
		out["serial"] = FieldChange{a.Serial, d.Serial}
	}
	if a.ModelName != d.ModelName && d.ModelName != "" {
		out["model_name"] = FieldChange{a.ModelName, d.ModelName}
	}
	for k, v := range d.Tags {
		if a.Tags[k] != v {
			out["tags."+k] = FieldChange{a.Tags[k], v}
		}
	}
	return out
}

// Diff classifies every host against the tenant's assets. It is pure and
// never panics on arbitrary input; the result is sorted by hostname.
func Diff(hosts []invclient.Host, assets []store.Asset) []Change {
	ix := buildIndex(assets)
	claimed := map[string]bool{}
	out := make([]Change, 0, len(hosts))
	for _, h := range hosts {
		d := desired(h)
		c := Change{HostID: h.ID, Hostname: h.Hostname, Serial: d.Serial}
		a, ok := ix.match(h)
		if ok && claimed[a.ID] {
			ok = false // two hosts resolving to one asset: the second creates
		}
		if !ok {
			c.Action = ActionCreate
			out = append(out, c)
			continue
		}
		claimed[a.ID] = true
		c.AssetID, c.AssetTag = a.ID, a.AssetTag
		c.Changes = diffOne(a, d)
		if len(c.Changes) == 0 {
			c.Action, c.Changes = ActionUnchanged, nil
		} else {
			c.Action = ActionUpdate
		}
		out = append(out, c)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Hostname < out[j].Hostname })
	return out
}

// applyDesired writes the desired projection onto an asset (sync-owned fields only).
func applyDesired(a *store.Asset, d Desired) {
	a.Name = d.Name
	if d.Serial != "" {
		a.Serial = d.Serial
	}
	if d.ModelName != "" {
		a.ModelName = d.ModelName
	}
	if a.Tags == nil {
		a.Tags = map[string]string{}
	}
	for k, v := range d.Tags {
		a.Tags[k] = v
	}
}
