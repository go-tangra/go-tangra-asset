package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// AssetAssignment records asset assignment/unassignment history
type AssetAssignment struct {
	ent.Schema
}

func (AssetAssignment) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "asset_assignments"},
		entsql.WithComments(true),
	}
}

func (AssetAssignment) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			NotEmpty().
			Unique().
			Comment("Unique identifier"),

		field.String("asset_id").
			NotEmpty().
			Comment("Asset ID"),

		field.String("asset_name").
			Optional().
			Comment("Asset name (denormalized)"),

		field.Uint32("user_id").
			Default(0).
			Comment("Portal user ID"),

		field.String("user_name").
			Optional().
			Comment("User display name (denormalized)"),

		field.Int32("action").
			Default(1).
			Comment("Action: 1=Assigned, 2=Unassigned, 3=Transferred"),

		field.Time("assigned_at").
			Comment("When the assignment was made"),

		field.Time("returned_at").
			Optional().
			Nillable().
			Comment("When the asset was returned"),

		field.Uint32("assigned_by").
			Optional().
			Nillable().
			Comment("User who performed the assignment"),

		field.Text("notes").
			Optional().
			Comment("Notes"),
	}
}

func (AssetAssignment) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("asset", Asset.Type).
			Ref("assignments").
			Field("asset_id").
			Unique().
			Required(),
	}
}

func (AssetAssignment) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("asset_id"),
		index.Fields("user_id"),
		index.Fields("asset_id", "returned_at"),
	}
}
