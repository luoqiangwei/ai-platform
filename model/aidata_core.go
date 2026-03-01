package model

import (
	"ai-platform/utils"
)

func CreateShotMemoryTable(dbClient *utils.SQLiteClient) error {
	fields := []utils.FieldDescription{
		{
			Name:            "id",
			DataType:        "INTEGER",
			IsPrimaryKey:    true,
			IsAutoIncrement: true,
		},
		{
			Name:         "user_id",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "message",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "response",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "timestamp",
			DataType:     "DATETIME",
			HasDefault:   true,
			DefaultValue: "CURRENT_TIMESTAMP",
		},
	}
	return dbClient.CreateTable("shot_memory", fields)
}

func CreateCoreMemoryTable(dbClient *utils.SQLiteClient) error {
	fields := []utils.FieldDescription{
		{
			Name:            "id",
			DataType:        "INTEGER",
			IsPrimaryKey:    true,
			IsAutoIncrement: true,
		},
		{
			Name:         "user_id",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "core_memory",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "timestamp",
			DataType:     "DATETIME",
			HasDefault:   true,
			DefaultValue: "CURRENT_TIMESTAMP",
		},
	}
	return dbClient.CreateTable("core_memory", fields)
}

func CreateTaskMemoryTable(dbClient *utils.SQLiteClient) error {
	fields := []utils.FieldDescription{
		{
			Name:            "id",
			DataType:        "INTEGER",
			IsPrimaryKey:    true,
			IsAutoIncrement: true,
		},
		{
			Name:         "user_id",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "task_description",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "due_date",
			DataType:     "DATETIME",
			IsPrimaryKey: false,
		},
		{
			Name:         "status",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "progress",
			DataType:     "INTEGER",
			IsPrimaryKey: false,
		},
		{
			Name:         "next_trigger_event",
			DataType:     "DATETIME",
			IsPrimaryKey: false,
		},
		{
			Name:         "timestamp",
			DataType:     "DATETIME",
			HasDefault:   true,
			DefaultValue: "CURRENT_TIMESTAMP",
		},
	}
	return dbClient.CreateTable("task_memory", fields)
}

func CreateAgentsTable(dbClient *utils.SQLiteClient) error {
	fields := []utils.FieldDescription{
		{
			Name:            "id",
			DataType:        "INTEGER",
			IsPrimaryKey:    true,
			IsAutoIncrement: true,
		},
		{
			Name:         "agent_name",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "agent_description",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "capabilities",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
	}
	return dbClient.CreateTable("agents", fields)
}

func CreateSkillsTable(dbClient *utils.SQLiteClient) error {
	fields := []utils.FieldDescription{
		{
			Name:            "id",
			DataType:        "INTEGER",
			IsPrimaryKey:    true,
			IsAutoIncrement: true,
		},
		{
			Name:         "skill_name",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "skill_description",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "restricted_scenarios",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
	}
	return dbClient.CreateTable("skills", fields)
}

func CreateBootstrapTable(dbClient *utils.SQLiteClient) error {
	fields := []utils.FieldDescription{
		{
			Name:            "id",
			DataType:        "INTEGER",
			IsPrimaryKey:    true,
			IsAutoIncrement: true,
		},
		{
			Name:         "context",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "risks",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "notes",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
	}
	return dbClient.CreateTable("bootstrap", fields)
}

func CreateHeadbeatTable(dbClient *utils.SQLiteClient) error {
	fields := []utils.FieldDescription{
		{
			Name:            "id",
			DataType:        "INTEGER",
			IsPrimaryKey:    true,
			IsAutoIncrement: true,
		},
		{
			Name:         "next_activation_time",
			DataType:     "DATETIME",
			IsPrimaryKey: false,
		},
		{
			Name:         "notes",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
	}
	return dbClient.CreateTable("headbeat", fields)
}

func CreateIdentityTable(dbClient *utils.SQLiteClient) error {
	fields := []utils.FieldDescription{
		{
			Name:            "id",
			DataType:        "INTEGER",
			IsPrimaryKey:    true,
			IsAutoIncrement: true,
		},
		{
			Name:         "identity_description",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "timestamp",
			DataType:     "DATETIME",
			HasDefault:   true,
			DefaultValue: "CURRENT_TIMESTAMP",
		},
	}
	return dbClient.CreateTable("identity", fields)
}

func CreateSoulTable(dbClient *utils.SQLiteClient) error {
	fields := []utils.FieldDescription{
		{
			Name:            "id",
			DataType:        "INTEGER",
			IsPrimaryKey:    true,
			IsAutoIncrement: true,
		},
		{
			Name:         "belief",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "timestamp",
			DataType:     "DATETIME",
			HasDefault:   true,
			DefaultValue: "CURRENT_TIMESTAMP",
		},
	}
	return dbClient.CreateTable("soul", fields)
}

func CreateToolsTable(dbClient *utils.SQLiteClient) error {
	fields := []utils.FieldDescription{
		{
			Name:            "id",
			DataType:        "INTEGER",
			IsPrimaryKey:    true,
			IsAutoIncrement: true,
		},
		{
			Name:         "tool_name",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "usage_instructions",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "usage_restrictions",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
	}
	return dbClient.CreateTable("tools", fields)
}

func CreateUserTable(dbClient *utils.SQLiteClient) error {
	fields := []utils.FieldDescription{
		{
			Name:            "id",
			DataType:        "INTEGER",
			IsPrimaryKey:    true,
			IsAutoIncrement: true,
		},
		{
			Name:         "user_id",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "attitude",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "thoughts",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "timestamp",
			DataType:     "DATETIME",
			HasDefault:   true,
			DefaultValue: "CURRENT_TIMESTAMP",
		},
	}
	return dbClient.CreateTable("user", fields)
}
