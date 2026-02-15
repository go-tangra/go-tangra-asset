package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// License represents a software license
type License struct {
	ent.Schema
}

func (License) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "asset_licenses"},
		entsql.WithComments(true),
	}
}

func (License) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			NotEmpty().
			Unique().
			Comment("Unique identifier"),

		field.String("name").
			NotEmpty().
			MaxLen(255).
			Comment("License name"),

		field.String("supplier_id").
			Optional().
			Comment("Supplier ID"),

		field.Time("purchase_date").
			Optional().
			Nillable().
			Comment("Purchase date"),

		field.Float("purchase_cost").
			Optional().
			Nillable().
			Comment("Purchase cost"),

		field.String("order_number").
			Optional().
			Comment("Order number"),

		field.Time("valid_from").
			Optional().
			Nillable().
			Comment("Valid from date"),

		field.Time("valid_to").
			Optional().
			Nillable().
			Comment("Valid to date"),

		field.Text("notes").
			Optional().
			Comment("Notes"),

		field.String("status").
			Optional().
			Default("ACTIVE").
			Comment("License status"),
	}
}

func (License) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("supplier", Supplier.Type).
			Field("supplier_id").
			Unique(),
	}
}

func (License) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.CreateBy{},
		mixin.UpdateBy{},
		mixin.Time{},
		mixin.TenantID[uint32]{},
	}
}

func (License) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "name"),
		index.Fields("tenant_id"),
		index.Fields("supplier_id"),
		index.Fields("status"),
	}
}
