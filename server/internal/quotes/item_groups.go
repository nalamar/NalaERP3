package quotes

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// QuoteItemGroup (LV-Hierarchie: Los/Titel/Untertitel) — Backlog B.1, siehe
// docs/adr/0009-lv-hierarchie-positionsmodell.md.
type QuoteItemGroup struct {
	ID            uuid.UUID  `json:"id"`
	QuoteID       uuid.UUID  `json:"quote_id"`
	ParentGroupID *uuid.UUID `json:"parent_group_id,omitempty"`
	Kind          string     `json:"kind"`
	Bezeichnung   string     `json:"bezeichnung"`
	SortOrder     int        `json:"sort_order"`
	CreatedAt     time.Time  `json:"created_at"`
}

type QuoteItemGroupCreate struct {
	ParentGroupID *uuid.UUID `json:"parent_group_id,omitempty"`
	Kind          string     `json:"kind"`
	Bezeichnung   string     `json:"bezeichnung"`
	SortOrder     int        `json:"sort_order"`
}

type QuoteItemGroupUpdate struct {
	Bezeichnung *string `json:"bezeichnung"`
	SortOrder   *int    `json:"sort_order"`
}

var validQuoteItemGroupKinds = map[string]bool{"los": true, "titel": true, "untertitel": true}

// ensureQuoteOwned prueft nur Existenz+Mandanten-Ownership (Leseoperationen),
// analog zum leichtgewichtigen Muster, das service.go bereits fuer
// Sub-Ressourcen-Zugriffe verwendet (z. B. ApplyMaterialCandidate), statt
// des vollstaendigen, deutlich teureren s.Get().
func (s *Service) ensureQuoteOwned(ctx context.Context, quoteID uuid.UUID, companyID string) error {
	var exists bool
	if err := s.pg.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM quotes WHERE id=$1 AND company_id=$2)`, quoteID, companyID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return errors.New("Angebot nicht gefunden")
	}
	return nil
}

// ensureQuoteEditable prueft zusaetzlich zur Ownership den bestehenden
// Schreibschutz aus Backlog 0.3.2 (nur draft-Angebote sind bearbeitbar,
// historische, per Revise ersetzte Versionen sind schreibgeschuetzt) -
// identisches Muster wie in ApplyMaterialCandidate u. a. Die LV-Struktur
// (Gruppen) ist ein struktureller Teil des Angebotsinhalts und unterliegt
// demselben Schutz wie die Positionen selbst.
func (s *Service) ensureQuoteEditable(ctx context.Context, quoteID uuid.UUID, companyID string) error {
	var status string
	var supersededByQuoteID uuid.NullUUID
	if err := s.pg.QueryRow(ctx, `SELECT status, superseded_by_quote_id FROM quotes WHERE id=$1 AND company_id=$2`, quoteID, companyID).Scan(&status, &supersededByQuoteID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("Angebot nicht gefunden")
		}
		return err
	}
	if supersededByQuoteID.Valid {
		return errors.New("Historische Angebotsversionen sind schreibgeschützt")
	}
	if status != "draft" {
		return errors.New("nur Entwürfe sind bearbeitbar")
	}
	return nil
}

// groupKind liest den kind eines Gruppenknotens, mandanten-/angebotsscharf
// ueber quoteID abgesichert (kein separates company_id auf
// quote_item_groups, Scope wird ueber quote_id geerbt, ADR 0009).
func (s *Service) groupKind(ctx context.Context, groupID, quoteID uuid.UUID) (string, error) {
	var kind string
	err := s.pg.QueryRow(ctx, `SELECT kind FROM quote_item_groups WHERE id=$1 AND quote_id=$2`, groupID, quoteID).Scan(&kind)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", errors.New("übergeordneter Knoten nicht gefunden")
		}
		return "", err
	}
	return kind, nil
}

// validateGroupHierarchy erzwingt die Wohlgeformtheits-Regel aus ADR 0009
// im Anwendungscode (bewusst kein DB-Trigger, analog ADR 0008): los hat nie
// einen Elternknoten, titel darf nur unter los oder auf oberster Ebene
// stehen, untertitel braucht zwingend einen titel-Elternknoten.
func (s *Service) validateGroupHierarchy(ctx context.Context, quoteID uuid.UUID, kind string, parentGroupID *uuid.UUID) error {
	if !validQuoteItemGroupKinds[kind] {
		return errors.New("kind ist ungültig (gültig: los, titel, untertitel)")
	}
	switch kind {
	case "los":
		if parentGroupID != nil {
			return errors.New("Ungültige Hierarchie: los darf keinen übergeordneten Knoten haben")
		}
	case "titel":
		if parentGroupID != nil {
			parentKind, err := s.groupKind(ctx, *parentGroupID, quoteID)
			if err != nil {
				return err
			}
			if parentKind != "los" {
				return errors.New("Ungültige Hierarchie: titel darf nur unter los oder auf oberster Ebene stehen")
			}
		}
	case "untertitel":
		if parentGroupID == nil {
			return errors.New("Ungültige Hierarchie: untertitel benötigt einen übergeordneten titel-Knoten")
		}
		parentKind, err := s.groupKind(ctx, *parentGroupID, quoteID)
		if err != nil {
			return err
		}
		if parentKind != "titel" {
			return errors.New("Ungültige Hierarchie: untertitel darf nur unter titel stehen")
		}
	}
	return nil
}

func (s *Service) CreateQuoteItemGroup(ctx context.Context, quoteID uuid.UUID, in QuoteItemGroupCreate, companyID string) (*QuoteItemGroup, error) {
	bezeichnung := strings.TrimSpace(in.Bezeichnung)
	if bezeichnung == "" {
		return nil, errors.New("bezeichnung erforderlich")
	}
	kind := strings.ToLower(strings.TrimSpace(in.Kind))
	if err := s.ensureQuoteEditable(ctx, quoteID, companyID); err != nil {
		return nil, err
	}
	if err := s.validateGroupHierarchy(ctx, quoteID, kind, in.ParentGroupID); err != nil {
		return nil, err
	}
	id := uuid.New()
	var g QuoteItemGroup
	err := s.pg.QueryRow(ctx, `
        INSERT INTO quote_item_groups (id, quote_id, parent_group_id, kind, bezeichnung, sort_order)
        VALUES ($1,$2,$3,$4,$5,$6)
        RETURNING id, quote_id, parent_group_id, kind, bezeichnung, sort_order, created_at
    `, id, quoteID, in.ParentGroupID, kind, bezeichnung, in.SortOrder).Scan(
		&g.ID, &g.QuoteID, &g.ParentGroupID, &g.Kind, &g.Bezeichnung, &g.SortOrder, &g.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func (s *Service) ListQuoteItemGroups(ctx context.Context, quoteID uuid.UUID, companyID string) ([]QuoteItemGroup, error) {
	if err := s.ensureQuoteOwned(ctx, quoteID, companyID); err != nil {
		return nil, err
	}
	rows, err := s.pg.Query(ctx, `
        SELECT id, quote_id, parent_group_id, kind, bezeichnung, sort_order, created_at
        FROM quote_item_groups WHERE quote_id=$1
        ORDER BY sort_order ASC, created_at ASC
    `, quoteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]QuoteItemGroup, 0)
	for rows.Next() {
		var g QuoteItemGroup
		if err := rows.Scan(&g.ID, &g.QuoteID, &g.ParentGroupID, &g.Kind, &g.Bezeichnung, &g.SortOrder, &g.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, nil
}

func (s *Service) UpdateQuoteItemGroup(ctx context.Context, quoteID, groupID uuid.UUID, in QuoteItemGroupUpdate, companyID string) (*QuoteItemGroup, error) {
	if err := s.ensureQuoteEditable(ctx, quoteID, companyID); err != nil {
		return nil, err
	}
	sets := make([]string, 0)
	args := make([]any, 0)
	idx := 1
	add := func(col string, v any) {
		sets = append(sets, fmt.Sprintf("%s=$%d", col, idx))
		args = append(args, v)
		idx++
	}
	if in.Bezeichnung != nil {
		bezeichnung := strings.TrimSpace(*in.Bezeichnung)
		if bezeichnung == "" {
			return nil, errors.New("bezeichnung erforderlich")
		}
		add("bezeichnung", bezeichnung)
	}
	if in.SortOrder != nil {
		add("sort_order", *in.SortOrder)
	}
	if len(sets) > 0 {
		args = append(args, groupID, quoteID)
		q := fmt.Sprintf("UPDATE quote_item_groups SET %s WHERE id=$%d AND quote_id=$%d", strings.Join(sets, ", "), idx, idx+1)
		tag, err := s.pg.Exec(ctx, q, args...)
		if err != nil {
			return nil, err
		}
		if tag.RowsAffected() == 0 {
			return nil, errors.New("Gruppe nicht gefunden")
		}
	}
	var g QuoteItemGroup
	err := s.pg.QueryRow(ctx, `
        SELECT id, quote_id, parent_group_id, kind, bezeichnung, sort_order, created_at
        FROM quote_item_groups WHERE id=$1 AND quote_id=$2
    `, groupID, quoteID).Scan(&g.ID, &g.QuoteID, &g.ParentGroupID, &g.Kind, &g.Bezeichnung, &g.SortOrder, &g.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("Gruppe nicht gefunden")
		}
		return nil, err
	}
	return &g, nil
}

func (s *Service) DeleteQuoteItemGroup(ctx context.Context, quoteID, groupID uuid.UUID, companyID string) error {
	if err := s.ensureQuoteEditable(ctx, quoteID, companyID); err != nil {
		return err
	}
	tag, err := s.pg.Exec(ctx, `DELETE FROM quote_item_groups WHERE id=$1 AND quote_id=$2`, groupID, quoteID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("Gruppe nicht gefunden")
	}
	return nil
}

// QuoteItemTreeNode ist ein Knoten im per GetQuoteItemTree assemblierten
// LV-Baum (Backlog B.1.3 "Baum-Assemblierungs-Helfer" aus ADR 0009).
type QuoteItemTreeNode struct {
	Group    QuoteItemGroup       `json:"group"`
	Children []*QuoteItemTreeNode `json:"children,omitempty"`
	Items    []QuoteItemInput     `json:"items,omitempty"`
}

// QuoteItemTree ist die vollstaendige, zur Leseseite assemblierte
// LV-Struktur eines Angebots.
type QuoteItemTree struct {
	Nodes          []*QuoteItemTreeNode `json:"nodes"`
	UngroupedItems []QuoteItemInput     `json:"ungrouped_items,omitempty"`
}

// GetQuoteItemTree baut die vollstaendige LV-Baumstruktur (Los/Titel/
// Untertitel + zugeordnete Positionen) fuer die Leseseite zusammen.
// Gruppen ohne Elternknoten bilden die Wurzeln; Positionen ohne group_id
// landen in UngroupedItems (bestehende, flache Angebote bleiben dadurch
// unveraendert lesbar). Sortierung: Gruppen nach sort_order (bereits von
// ListQuoteItemGroups geliefert), Positionen nach der bestehenden
// position-Spalte (bereits von Get gelieferte Reihenfolge) - beide
// Reihenfolgen bleiben beim Aufbau des Baums erhalten (append bewahrt die
// Eingabereihenfolge je Elternknoten/Gruppe).
func (s *Service) GetQuoteItemTree(ctx context.Context, quoteID uuid.UUID, companyID string) (*QuoteItemTree, error) {
	if err := s.ensureQuoteOwned(ctx, quoteID, companyID); err != nil {
		return nil, err
	}
	groups, err := s.ListQuoteItemGroups(ctx, quoteID, companyID)
	if err != nil {
		return nil, err
	}
	quote, err := s.Get(ctx, quoteID, companyID)
	if err != nil {
		return nil, err
	}
	return buildQuoteItemTree(groups, quote.Items), nil
}

func buildQuoteItemTree(groups []QuoteItemGroup, items []QuoteItemInput) *QuoteItemTree {
	nodeByID := make(map[uuid.UUID]*QuoteItemTreeNode, len(groups))
	for _, g := range groups {
		nodeByID[g.ID] = &QuoteItemTreeNode{Group: g}
	}
	tree := &QuoteItemTree{Nodes: make([]*QuoteItemTreeNode, 0)}
	for _, g := range groups {
		node := nodeByID[g.ID]
		if g.ParentGroupID != nil {
			if parent, ok := nodeByID[*g.ParentGroupID]; ok {
				parent.Children = append(parent.Children, node)
				continue
			}
		}
		tree.Nodes = append(tree.Nodes, node)
	}
	for _, item := range items {
		if strings.TrimSpace(item.GroupID) == "" {
			tree.UngroupedItems = append(tree.UngroupedItems, item)
			continue
		}
		groupID, err := uuid.Parse(item.GroupID)
		if err != nil {
			tree.UngroupedItems = append(tree.UngroupedItems, item)
			continue
		}
		if node, ok := nodeByID[groupID]; ok {
			node.Items = append(node.Items, item)
		} else {
			tree.UngroupedItems = append(tree.UngroupedItems, item)
		}
	}
	return tree
}
