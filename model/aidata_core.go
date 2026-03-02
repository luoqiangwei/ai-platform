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
			Name:         "message_id",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "chat_id",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "channel",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "created_at",
			DataType:     "DATETIME",
			HasDefault:   true,
			DefaultValue: "CURRENT_TIMESTAMP",
		},
		{
			Name:         "updated_at",
			DataType:     "DATETIME",
			HasDefault:   true,
			DefaultValue: "CURRENT_TIMESTAMP",
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
			Name:         "source",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "created_at",
			DataType:     "DATETIME",
			HasDefault:   true,
			DefaultValue: "CURRENT_TIMESTAMP",
		},
		{
			Name:         "updated_at",
			DataType:     "DATETIME",
			HasDefault:   true,
			DefaultValue: "CURRENT_TIMESTAMP",
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
			Name:         "category",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "due_date",
			DataType:     "DATETIME",
			IsPrimaryKey: false,
		},
		{
			Name:         "expires_at",
			DataType:     "DATETIME",
			IsPrimaryKey: false,
		},
		{
			Name:         "status",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "priority",
			DataType:     "INTEGER",
			HasDefault:   true,
			DefaultValue: "0",
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
			Name:         "metadata",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "created_at",
			DataType:     "DATETIME",
			HasDefault:   true,
			DefaultValue: "CURRENT_TIMESTAMP",
		},
		{
			Name:         "updated_at",
			DataType:     "DATETIME",
			HasDefault:   true,
			DefaultValue: "CURRENT_TIMESTAMP",
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
		{
			Name:         "category",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "tags",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "source",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "version",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "expires_at",
			DataType:     "DATETIME",
			IsPrimaryKey: false,
		},
		{
			Name:         "path",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "entrypoint",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "enabled",
			DataType:     "INTEGER",
			HasDefault:   true,
			DefaultValue: "1",
		},
		{
			Name:         "created_at",
			DataType:     "DATETIME",
			HasDefault:   true,
			DefaultValue: "CURRENT_TIMESTAMP",
		},
		{
			Name:         "updated_at",
			DataType:     "DATETIME",
			HasDefault:   true,
			DefaultValue: "CURRENT_TIMESTAMP",
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
		{
			Name:         "category",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "tags",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "source",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "version",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "expires_at",
			DataType:     "DATETIME",
			IsPrimaryKey: false,
		},
		{
			Name:         "path",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "entrypoint",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "enabled",
			DataType:     "INTEGER",
			HasDefault:   true,
			DefaultValue: "1",
		},
		{
			Name:         "created_at",
			DataType:     "DATETIME",
			HasDefault:   true,
			DefaultValue: "CURRENT_TIMESTAMP",
		},
		{
			Name:         "updated_at",
			DataType:     "DATETIME",
			HasDefault:   true,
			DefaultValue: "CURRENT_TIMESTAMP",
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
		{
			Name:         "category",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "tags",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "source",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "version",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "expires_at",
			DataType:     "DATETIME",
			IsPrimaryKey: false,
		},
		{
			Name:         "path",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "entrypoint",
			DataType:     "TEXT",
			IsPrimaryKey: false,
		},
		{
			Name:         "enabled",
			DataType:     "INTEGER",
			HasDefault:   true,
			DefaultValue: "1",
		},
		{
			Name:         "created_at",
			DataType:     "DATETIME",
			HasDefault:   true,
			DefaultValue: "CURRENT_TIMESTAMP",
		},
		{
			Name:         "updated_at",
			DataType:     "DATETIME",
			HasDefault:   true,
			DefaultValue: "CURRENT_TIMESTAMP",
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

func CreateToolRunsTable(dbClient *utils.SQLiteClient) error {
	fields := []utils.FieldDescription{
		{
			Name:            "id",
			DataType:        "INTEGER",
			IsPrimaryKey:    true,
			IsAutoIncrement: true,
		},
		{
			Name:     "user_id",
			DataType: "TEXT",
		},
		{
			Name:     "tool",
			DataType: "TEXT",
		},
		{
			Name:     "command",
			DataType: "TEXT",
		},
		{
			Name:     "args",
			DataType: "TEXT",
		},
		{
			Name:     "stdout",
			DataType: "TEXT",
		},
		{
			Name:     "stderr",
			DataType: "TEXT",
		},
		{
			Name:     "exit_code",
			DataType: "INTEGER",
		},
		{
			Name:         "created_at",
			DataType:     "DATETIME",
			HasDefault:   true,
			DefaultValue: "CURRENT_TIMESTAMP",
		},
	}
	return dbClient.CreateTable("tool_runs", fields)
}

func EnsureAIBrainSchema(dbClient *utils.SQLiteClient) error {
	if err := dbClient.EnsureColumns("shot_memory", []utils.FieldDescription{
		{Name: "message_id", DataType: "TEXT"},
		{Name: "chat_id", DataType: "TEXT"},
		{Name: "channel", DataType: "TEXT"},
		{Name: "created_at", DataType: "DATETIME", HasDefault: true, DefaultValue: "CURRENT_TIMESTAMP"},
		{Name: "updated_at", DataType: "DATETIME", HasDefault: true, DefaultValue: "CURRENT_TIMESTAMP"},
	}); err != nil {
		return err
	}

	if err := dbClient.EnsureColumns("core_memory", []utils.FieldDescription{
		{Name: "source", DataType: "TEXT"},
		{Name: "created_at", DataType: "DATETIME", HasDefault: true, DefaultValue: "CURRENT_TIMESTAMP"},
		{Name: "updated_at", DataType: "DATETIME", HasDefault: true, DefaultValue: "CURRENT_TIMESTAMP"},
	}); err != nil {
		return err
	}

	if err := dbClient.EnsureColumns("task_memory", []utils.FieldDescription{
		{Name: "category", DataType: "TEXT"},
		{Name: "expires_at", DataType: "DATETIME"},
		{Name: "priority", DataType: "INTEGER", HasDefault: true, DefaultValue: "0"},
		{Name: "metadata", DataType: "TEXT"},
		{Name: "created_at", DataType: "DATETIME", HasDefault: true, DefaultValue: "CURRENT_TIMESTAMP"},
		{Name: "updated_at", DataType: "DATETIME", HasDefault: true, DefaultValue: "CURRENT_TIMESTAMP"},
	}); err != nil {
		return err
	}

	shared := []utils.FieldDescription{
		{Name: "category", DataType: "TEXT"},
		{Name: "tags", DataType: "TEXT"},
		{Name: "source", DataType: "TEXT"},
		{Name: "version", DataType: "TEXT"},
		{Name: "expires_at", DataType: "DATETIME"},
		{Name: "path", DataType: "TEXT"},
		{Name: "entrypoint", DataType: "TEXT"},
		{Name: "enabled", DataType: "INTEGER", HasDefault: true, DefaultValue: "1"},
		{Name: "created_at", DataType: "DATETIME", HasDefault: true, DefaultValue: "CURRENT_TIMESTAMP"},
		{Name: "updated_at", DataType: "DATETIME", HasDefault: true, DefaultValue: "CURRENT_TIMESTAMP"},
	}

	if err := dbClient.EnsureColumns("agents", shared); err != nil {
		return err
	}
	if err := dbClient.EnsureColumns("skills", shared); err != nil {
		return err
	}
	if err := dbClient.EnsureColumns("tools", shared); err != nil {
		return err
	}

	if err := dbClient.CreateTable("tool_runs", []utils.FieldDescription{
		{Name: "id", DataType: "INTEGER", IsPrimaryKey: true, IsAutoIncrement: true},
		{Name: "user_id", DataType: "TEXT"},
		{Name: "tool", DataType: "TEXT"},
		{Name: "command", DataType: "TEXT"},
		{Name: "args", DataType: "TEXT"},
		{Name: "stdout", DataType: "TEXT"},
		{Name: "stderr", DataType: "TEXT"},
		{Name: "exit_code", DataType: "INTEGER"},
		{Name: "created_at", DataType: "DATETIME", HasDefault: true, DefaultValue: "CURRENT_TIMESTAMP"},
	}); err != nil {
		return err
	}

	return nil
}
