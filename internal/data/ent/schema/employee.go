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

// Employee represents a person who can be assigned assets
type Employee struct {
	ent.Schema
}

func (Employee) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "asset_employees"},
		entsql.WithComments(true),
	}
}

func (Employee) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			NotEmpty().
			Unique().
			Comment("Unique identifier"),

		field.String("first_name").
			NotEmpty().
			MaxLen(255).
			Comment("First name"),

		field.String("last_name").
			NotEmpty().
			MaxLen(255).
			Comment("Last name"),

		field.String("email").
			Optional().
			Comment("Email address"),

		field.String("phone").
			Optional().
			Comment("Phone number"),

		field.String("department").
			Optional().
			Comment("Department"),

		field.String("job_title").
			Optional().
			Comment("Job title"),

		field.String("employee_number").
			Optional().
			Comment("Employee number"),

		field.Text("notes").
			Optional().
			Comment("Notes"),

		field.Text("tags").
			Optional().
			Comment("Custom tags (JSON)"),

		field.JSON("metadata", map[string]interface{}{}).
			Optional().
			Comment("Custom metadata (JSON)"),
	}
}

func (Employee) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("assets", Asset.Type).
			Ref("employee"),
		edge.From("assignments", AssetAssignment.Type).
			Ref("employee"),
	}
}

func (Employee) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.CreateBy{},
		mixin.UpdateBy{},
		mixin.Time{},
		mixin.TenantID[uint32]{},
	}
}

func (Employee) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "email").Unique(),
		index.Fields("tenant_id", "employee_number").Unique(),
		index.Fields("tenant_id"),
		index.Fields("department"),
	}
}
