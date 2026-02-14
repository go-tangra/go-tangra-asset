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

// Asset represents an IT asset (laptop, PC, printer, etc.)
type Asset struct {
	ent.Schema
}

func (Asset) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "asset_assets"},
		entsql.WithComments(true),
	}
}

func (Asset) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			NotEmpty().
			Unique().
			Comment("Unique identifier"),

		field.String("asset_tag").
			NotEmpty().
			MaxLen(255).
			Comment("Asset tag (unique per tenant)"),

		field.String("name").
			Optional().
			Default("").
			MaxLen(255).
			Comment("Asset name"),

		field.String("serial").
			Optional().
			Comment("Serial number"),

		field.String("model_name").
			Optional().
			Comment("Model name"),

		field.String("model_number").
			Optional().
			Comment("Model number"),

		field.String("category_id").
			Optional().
			Comment("Category ID"),

		field.String("supplier_id").
			Optional().
			Comment("Supplier ID"),

		field.String("location_id").
			Optional().
			Comment("Location ID (when unassigned)"),

		field.String("employee_id").
			Optional().
			Comment("Employee ID (when assigned)"),

		field.Int32("status").
			Default(1).
			Comment("Asset status: 1=Deployable, 2=Assigned, 3=Broken, 4=Archived"),

		field.String("photo_key").
			Optional().
			Comment("Photo storage key in S3"),

		field.Int32("warranty_months").
			Optional().
			Nillable().
			Comment("Warranty duration in months"),

		field.Time("purchase_date").
			Optional().
			Nillable().
			Comment("Purchase date"),

		field.String("order_number").
			Optional().
			Comment("Order number"),

		field.Float("purchase_cost").
			Optional().
			Nillable().
			Comment("Purchase cost"),

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

func (Asset) Edges() []ent.Edge {
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
		edge.To("employee", Employee.Type).
			Field("employee_id").
			Unique(),
		edge.To("assignments", AssetAssignment.Type),
	}
}

func (Asset) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.CreateBy{},
		mixin.UpdateBy{},
		mixin.Time{},
		mixin.TenantID[uint32]{},
	}
}

func (Asset) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "asset_tag").Unique(),
		index.Fields("tenant_id", "serial"),
		index.Fields("tenant_id"),
		index.Fields("status"),
		index.Fields("category_id"),
		index.Fields("supplier_id"),
		index.Fields("employee_id"),
		index.Fields("location_id"),
	}
}
