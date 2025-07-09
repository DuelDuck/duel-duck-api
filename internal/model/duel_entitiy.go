package model

import (
	"github.com/uptrace/bun"
)

type DuelTopic struct {
	bun.BaseModel `bun:"table:duel_topics,alias:duel_topics" json:"-"`

	ID       uint8  `bun:",pk,autoincrement"`
	Name     string `bun:"name,notnull,unique"`
	ImageURL string `bun:"image_url,type:image_url"`
}

type DuelSubtopic struct {
	bun.BaseModel `bun:"table:duel_subtopics,alias:duel_subtopics" json:"-"`

	ID       uint64 `bun:"id,pk,autoincrement" json:"id"`
	TopicID  uint64 `bun:"topic_id,notnull" json:"topic_id"`
	Name     string `bun:"name,type:varchar(32),notnull,unique" json:"name"`
	ImageURL string `bun:"image_url,type:image_url" json:"image_url"`
}

type DuelEntity struct {
	bun.BaseModel `bun:"table:duel_entities,alias:duel_entities" json:"-"`

	ID           uint64 `bun:"id,pk,autoincrement" json:"id"`
	SubtopicID   uint64 `bun:"subtopic_id,notnull" json:"subtopic_id"`
	Name         string `bun:"name,type:varchar(32),unique" json:"name"`
	EntityTypeID uint64 `bun:"entity_type_id,type:integer" json:"entity_type_id"`
	ImageURL     string `bun:"image_url" json:"image_url"`
}

type DuelType struct {
	bun.BaseModel `bun:"table:duel_types,alias:duel_types" json:"-"`

	ID   uint8  `bun:"id,pk,autoincrement"`
	Type string `bun:"type,notnull,unique"`
}

type CreateDuelEntityReq struct {
	SubtopicID   uint64 `json:"subtopic_id"`
	Name         string `json:"name"`
	EntityTypeID uint64 `json:"entity_type_id"`
	ImageURL     string `json:"image_url"`
}

type CreateDuelSubtopicReq struct {
	TopicID  uint64 `json:"topic_id"`
	Name     string `json:"name"`
	ImageURL string `json:"image_url"`
}
