package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// InsurancePolicy represents an insurance policy
type InsurancePolicy struct {
	ent.Schema
}

func (InsurancePolicy) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "asset_insurance_policies"},
		entsql.WithComments(true),
	}
}

func (InsurancePolicy) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			NotEmpty().
			Unique().
			Comment("Unique identifier"),

		field.String("name").
			NotEmpty().
			MaxLen(255).
			Comment("Policy name"),

		field.String("policy_number").
			Optional().
			Comment("Policy number"),

		field.String("provider").
			Optional().
			Comment("Insurance provider"),

		field.String("coverage_type").
			Optional().
			Comment("Coverage type"),

		field.Float("premium_amount").
			Optional().
			Nillable().
			Comment("Premium amount"),

		field.Float("deductible").
			Optional().
			Nillable().
			Comment("Deductible"),

		field.Float("coverage_limit").
			Optional().
			Nillable().
			Comment("Coverage limit"),

		field.Time("valid_from").
			Optional().
			Nillable().
			Comment("Valid from date"),

		field.Time("valid_to").
			Optional().
			Nillable().
			Comment("Valid to date"),

		field.String("status").
			Optional().
			Default("ACTIVE").
			Comment("Policy status"),

		field.Text("notes").
			Optional().
			Comment("Notes"),

		field.JSON("metadata", map[string]interface{}{}).
			Optional().
			Comment("Custom metadata (JSON)"),
	}
}

func (InsurancePolicy) Edges() []ent.Edge {
	return nil
}

func (InsurancePolicy) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.CreateBy{},
		mixin.UpdateBy{},
		mixin.Time{},
		mixin.TenantID[uint32]{},
	}
}

func (InsurancePolicy) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "policy_number").Unique(),
		index.Fields("tenant_id", "name"),
		index.Fields("tenant_id"),
		index.Fields("status"),
	}
}
