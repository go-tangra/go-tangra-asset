package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// InsurancePolicyAsset represents the many-to-many relationship between policies and assets
type InsurancePolicyAsset struct {
	ent.Schema
}

func (InsurancePolicyAsset) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "asset_insurance_policy_assets"},
		entsql.WithComments(true),
	}
}

func (InsurancePolicyAsset) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			NotEmpty().
			Unique().
			Comment("Unique identifier"),

		field.String("policy_id").
			NotEmpty().
			Comment("Insurance policy ID"),

		field.String("asset_id").
			NotEmpty().
			Comment("Asset ID"),

		field.Uint32("tenant_id").
			Comment("Tenant ID"),

		field.Float("covered_value").
			Optional().
			Nillable().
			Comment("Covered value for this asset"),

		field.Text("notes").
			Optional().
			Comment("Notes"),

		field.String("asset_tag").
			Optional().
			Comment("Denormalized asset tag"),

		field.String("asset_name").
			Optional().
			Comment("Denormalized asset name"),

		field.String("model_name").
			Optional().
			Comment("Denormalized model name"),
	}
}

func (InsurancePolicyAsset) Edges() []ent.Edge {
	return nil
}

func (InsurancePolicyAsset) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.CreateBy{},
		mixin.Time{},
	}
}

func (InsurancePolicyAsset) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("policy_id", "asset_id").Unique(),
		index.Fields("policy_id"),
		index.Fields("asset_id"),
		index.Fields("tenant_id"),
	}
}
