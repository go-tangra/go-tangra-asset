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

// Category represents an asset/consumable classification
type Category struct {
	ent.Schema
}

func (Category) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "asset_categories"},
		entsql.WithComments(true),
	}
}

func (Category) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			NotEmpty().
			Unique().
			Comment("Unique identifier"),

		field.String("name").
			NotEmpty().
			MaxLen(255).
			Comment("Category name"),

		field.String("description").
			Optional().
			Comment("Description"),

		field.String("parent_id").
			Optional().
			Comment("Parent category ID (for hierarchy)"),

		field.String("icon").
			Optional().
			Comment("Icon name"),

		field.JSON("metadata", map[string]interface{}{}).
			Optional().
			Comment("Custom metadata (JSON)"),
	}
}

func (Category) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("children", Category.Type).
			From("parent").
			Field("parent_id").
			Unique(),
		edge.From("assets", Asset.Type).
			Ref("category"),
		edge.From("consumables", Consumable.Type).
			Ref("category"),
	}
}

func (Category) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.CreateBy{},
		mixin.UpdateBy{},
		mixin.Time{},
		mixin.TenantID[uint32]{},
	}
}

func (Category) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "name", "parent_id").Unique().StorageKey("asset_category_tenant_name_parent"),
		index.Fields("tenant_id").StorageKey("asset_category_tenant_id"),
		index.Fields("parent_id").StorageKey("asset_category_parent_id"),
	}
}
