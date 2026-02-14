package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/tx7do/go-crud/entgo/mixin"
)

// AssetDocument represents a file attached to an asset or consumable (polymorphic)
type AssetDocument struct {
	ent.Schema
}

func (AssetDocument) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "asset_documents"},
		entsql.WithComments(true),
	}
}

func (AssetDocument) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			NotEmpty().
			Unique().
			Comment("Unique identifier"),

		field.String("entity_type").
			NotEmpty().
			Comment("Entity type: asset or consumable"),

		field.String("entity_id").
			NotEmpty().
			Comment("Entity ID (asset ID or consumable ID)"),

		field.String("file_name").
			NotEmpty().
			Comment("Original file name"),

		field.Int64("file_size").
			Default(0).
			Comment("File size in bytes"),

		field.String("mime_type").
			Optional().
			Comment("MIME type"),

		field.String("storage_key").
			NotEmpty().
			Comment("Storage key in S3"),

		field.String("checksum").
			Optional().
			Comment("SHA-256 checksum"),

		field.String("description").
			Optional().
			Comment("Description"),

		field.Uint32("uploaded_by").
			Optional().
			Nillable().
			Comment("User who uploaded the document"),
	}
}

func (AssetDocument) Edges() []ent.Edge {
	return nil
}

func (AssetDocument) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.Time{},
	}
}

func (AssetDocument) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("entity_type", "entity_id"),
		index.Fields("storage_key").Unique(),
	}
}
