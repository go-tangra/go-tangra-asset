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

// Consumable represents a consumable item (keyboards, mice, cables, etc.)
type Consumable struct {
	ent.Schema
}

func (Consumable) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "asset_consumables"},
		entsql.WithComments(true),
	}
}

func (Consumable) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			NotEmpty().
			Unique().
			Comment("Unique identifier"),

		field.String("name").
			NotEmpty().
			MaxLen(255).
			Comment("Consumable name"),

		field.String("description").
			Optional().
			Comment("Description"),

		field.String("category_id").
			Optional().
			Comment("Category ID"),

		field.String("supplier_id").
			Optional().
			Comment("Supplier ID"),

		field.String("location_id").
			Optional().
			Comment("Location ID"),

		field.String("model_name").
			Optional().
			Comment("Model name"),

		field.String("model_number").
			Optional().
			Comment("Model number"),

		field.Int32("amount").
			Default(0).
			Comment("Current quantity"),

		field.Int32("min_amount").
			Default(0).
			Comment("Minimum quantity threshold"),

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

		field.Text("notes").
			Optional().
			Comment("Notes"),

		field.Text("tags").
			Optional().
			Comment("Custom tags (JSON)"),

		field.Text("metadata").
			Optional().
			Comment("Custom metadata (JSON)"),
	}
}

func (Consumable) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("category", Category.Type).
			Field("category_id").
			Unique(),
		edge.To("supplier", Supplier.Type).
			Field("supplier_id").
			Unique(),
		edge.To("location", Location.Type).
			Field("location_id").
			Unique(),
	}
}

func (Consumable) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.CreateBy{},
		mixin.UpdateBy{},
		mixin.Time{},
		mixin.TenantID[uint32]{},
	}
}

func (Consumable) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "name"),
		index.Fields("tenant_id"),
		index.Fields("category_id"),
		index.Fields("supplier_id"),
		index.Fields("location_id"),
	}
}
