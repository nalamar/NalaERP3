// Package auditlog implementiert das generische Änderungsprotokoll über
// Fachobjekte (Task 0.3.3). entity_type/entity_id sind bewusst frei statt
// über feste Fremdschlüssel an eine Domäne gebunden, damit dieselbe
// Tabelle von beliebigen Domänen genutzt werden kann.
package auditlog

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Entry struct {
	ID          uuid.UUID `json:"id"`
	EntityType  string    `json:"entity_type"`
	EntityID    string    `json:"entity_id"`
	Action      string    `json:"action"`
	ActorUserID *string   `json:"actor_user_id,omitempty"`
	Before      any       `json:"before,omitempty"`
	After       any       `json:"after,omitempty"`
	Note        string    `json:"note,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type RecordInput struct {
	EntityType  string
	EntityID    string
	Action      string
	ActorUserID string // leer = unbekannt/System
	Before      any
	After       any
	Note        string
}

type Service struct{ pg *pgxpool.Pool }

func NewService(pg *pgxpool.Pool) *Service { return &Service{pg: pg} }

// Record schreibt einen Protokolleintrag. Nimmt optional eine bereits
// laufende Transaktion entgegen, damit der Eintrag atomar zusammen mit der
// eigentlichen fachlichen Änderung geschrieben (und bei einem Rollback der
// Änderung ebenfalls zurückgerollt) wird.
func (s *Service) Record(ctx context.Context, tx pgx.Tx, companyID string, in RecordInput) error {
	if strings.TrimSpace(companyID) == "" {
		return errors.New("Mandant erforderlich")
	}
	if strings.TrimSpace(in.EntityType) == "" {
		return errors.New("entity_type erforderlich")
	}
	if strings.TrimSpace(in.EntityID) == "" {
		return errors.New("entity_id erforderlich")
	}
	if strings.TrimSpace(in.Action) == "" {
		return errors.New("action erforderlich")
	}
	beforeJSON, err := marshalOrNil(in.Before)
	if err != nil {
		return err
	}
	afterJSON, err := marshalOrNil(in.After)
	if err != nil {
		return err
	}
	id := uuid.New()
	var actorUserID *string
	if strings.TrimSpace(in.ActorUserID) != "" {
		actorUserID = &in.ActorUserID
	}

	q := `INSERT INTO entity_change_log (id, company_id, entity_type, entity_id, action, actor_user_id, before_data, after_data, note)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`
	args := []any{id, companyID, in.EntityType, in.EntityID, in.Action, actorUserID, beforeJSON, afterJSON, in.Note}
	if tx != nil {
		_, err = tx.Exec(ctx, q, args...)
	} else {
		_, err = s.pg.Exec(ctx, q, args...)
	}
	return err
}

// List liefert den Änderungsverlauf eines Fachobjekts, neueste zuerst.
func (s *Service) List(ctx context.Context, entityType, entityID, companyID string) ([]Entry, error) {
	if strings.TrimSpace(companyID) == "" {
		return nil, errors.New("Mandant erforderlich")
	}
	rows, err := s.pg.Query(ctx, `
		SELECT id, entity_type, entity_id, action, actor_user_id, before_data, after_data, note, created_at
		FROM entity_change_log
		WHERE entity_type=$1 AND entity_id=$2 AND company_id=$3
		ORDER BY created_at DESC, id DESC
	`, entityType, entityID, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Entry, 0)
	for rows.Next() {
		var e Entry
		var actorUserID *string
		var before, after []byte
		if err := rows.Scan(&e.ID, &e.EntityType, &e.EntityID, &e.Action, &actorUserID, &before, &after, &e.Note, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.ActorUserID = actorUserID
		if len(before) > 0 {
			var v any
			if err := json.Unmarshal(before, &v); err == nil {
				e.Before = v
			}
		}
		if len(after) > 0 {
			var v any
			if err := json.Unmarshal(after, &v); err == nil {
				e.After = v
			}
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func marshalOrNil(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}
