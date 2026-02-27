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

// Supplier represents an asset/consumable vendor
type Supplier struct {
	ent.Schema
}

func (Supplier) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "asset_suppliers"},
		entsql.WithComments(true),
	}
}

func (Supplier) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			NotEmpty().
			Unique().
			Comment("Unique identifier"),

		field.String("name").
			NotEmpty().
			MaxLen(255).
			Comment("Supplier name"),

		field.String("code").
			Optional().
			MaxLen(50).
			Comment("Supplier code (short identifier)"),

		field.String("address").
			Optional().
			Comment("Physical address"),

		field.String("city").
			Optional().
			Comment("City"),

		field.String("state").
			Optional().
			Comment("State/Province"),

		field.String("country").
			Optional().
			Comment("Country"),

		field.String("postal_code").
			Optional().
			Comment("Postal code"),

		field.String("contact_person").
			Optional().
			Comment("Contact person"),

		field.String("telephone").
			Optional().
			Comment("Telephone"),

		field.String("email").
			Optional().
			Comment("Email"),

		field.String("website").
			Optional().
			Comment("Website"),

		field.Text("notes").
			Optional().
			Comment("Notes"),

		field.Text("tags").
			Optional().
			Comment("Custom tags (JSON)"),

		field.JSON("metadata", map[string]interface{}{}).
			Optional().
			Comment("Custom metadata (JSON)"),

		field.Int32("status").
			Default(1).
			Comment("Supplier status: 1=Active, 2=Inactive"),
	}
}

func (Supplier) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("assets", Asset.Type).
			Ref("supplier"),
		edge.From("consumables", Consumable.Type).
			Ref("supplier"),
	}
}

func (Supplier) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.CreateBy{},
		mixin.UpdateBy{},
		mixin.Time{},
		mixin.TenantID[uint32]{},
	}
}

func (Supplier) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "name").Unique(),
		index.Fields("tenant_id"),
		index.Fields("status"),
	}
}
