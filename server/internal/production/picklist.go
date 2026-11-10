package production

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Kommissionierliste je Auftrag (ADR 0038 Frage 4 (a), Backlog H.3.3a):
// was noch aus dem Lager zu holen ist, mit Lager und Platz, ohne Preise.
// Das PDF baut pdfgen (H.3.3b).

// MaxPickPlaces: so viele Lagerplaetze je Zeile, der Rest als Anzahl.
const MaxPickPlaces = 3

var errPickListStatus = errors.New("Kommissionierliste nur bei freigegebenen oder begonnenen Aufträgen")

type PickList struct {
	Nummer         string     `json:"nummer"`
	ProjectNummer  string     `json:"project_nummer"`
	ProjectName    string     `json:"project_name"`
	PositionNummer string     `json:"position_nummer"`
	PositionName   string     `json:"position_name"`
	VarianteName   string     `json:"variante_name"`
	Stueckzahl     int        `json:"stueckzahl"`
	Liefertermin   *time.Time `json:"liefertermin"`
	Status         string     `json:"status"`
	// Reservierungslager "Code – Name"; leer ohne Reservierung.
	Lager string     `json:"lager"`
	Lines []PickLine `json:"lines"`
}

// PickLine: eine noch offene Zeile der Stueckliste. Mit umrechenbarem
// Bedarf ist Menge die offene Menge in der Einheit des Materials, sonst
// der Gesamtbedarf in der Einheit der Zeile (Gesamt=true).
type PickLine struct {
	Pos                 int      `json:"pos"`
	Art                 string   `json:"art"`
	ArticleCode         string   `json:"article_code"`
	Description         string   `json:"description"`
	MaterialNummer      string   `json:"material_nummer"`
	MaterialBezeichnung string   `json:"material_bezeichnung"`
	Abmessung           string   `json:"abmessung"`
	Menge               float64  `json:"menge"`
	Einheit             string   `json:"einheit"`
	Gesamt              bool     `json:"gesamt"`
	Plaetze             []string `json:"plaetze"`
}

// stockPlace: Bestand eines Materials an einem Platz (location leer: im
// Lager ohne Platz).
type stockPlace struct {
	lager, platz string
	qty          float64
}

// PickAbmessung: Profil mit Laenge, Glas mit Breite x Hoehe, sonst leer.
func PickAbmessung(it Item) string {
	switch {
	case it.Art == ArtProfil && it.LengthMM != nil:
		return "L " + formatMM(*it.LengthMM) + " mm"
	case it.Art == ArtGlas && it.WidthMM != nil && it.HeightMM != nil:
		return formatMM(*it.WidthMM) + " × " + formatMM(*it.HeightMM) + " mm"
	}
	return ""
}

func formatMM(v float64) string {
	return strings.Replace(strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.1f", v), "0"), "."), ".", ",", 1)
}

// PickQty: offene Menge (Material, umrechenbar) oder Gesamtbedarf der
// Zeile; keep=false fuer eine vollstaendig entnommene Zeile.
func PickQty(it Item) (menge float64, einheit string, gesamt, keep bool) {
	if it.Offen != nil {
		return *it.Offen, it.MaterialEinheit, false, *it.Offen > 0
	}
	return round6(it.MengeGesamt), it.Unit, true, true
}

// FormatPickPlaces: Plaetze mit Bestand, groesster zuerst, hoechstens max,
// danach "+ n weitere". Ohne Bestand "kein Bestand".
func FormatPickPlaces(places []stockPlace, einheit string, max int) []string {
	var pos []stockPlace
	for _, p := range places {
		if round6(p.qty) > 0 {
			pos = append(pos, p)
		}
	}
	if len(pos) == 0 {
		return []string{"kein Bestand"}
	}
	sort.SliceStable(pos, func(i, j int) bool {
		if pos[i].qty != pos[j].qty {
			return pos[i].qty > pos[j].qty
		}
		return pos[i].lager+pos[i].platz < pos[j].lager+pos[j].platz
	})
	out := make([]string, 0, max+1)
	for i, p := range pos {
		if i == max {
			out = append(out, fmt.Sprintf("+ %d weitere", len(pos)-max))
			break
		}
		where := p.lager
		if p.platz != "" {
			where += " / " + p.platz
		}
		out = append(out, strings.TrimSpace(where+": "+formatQty(p.qty)+" "+einheit))
	}
	return out
}

func formatQty(v float64) string {
	s := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.3f", round6(v)), "0"), ".")
	return strings.Replace(s, ".", ",", 1)
}

// PickList liefert die Kommissionierliste eines freigegebenen oder
// begonnenen Auftrags. Plaetze aus dem Reservierungslager, ohne
// Reservierung aus allen Lagern des Mandanten.
func (s *Service) PickList(ctx context.Context, id uuid.UUID, companyID string) (*PickList, error) {
	x, err := s.Get(ctx, id, companyID)
	if err != nil {
		return nil, err
	}
	if err := WithdrawalAllowed(x.Status); err != nil {
		return nil, errPickListStatus
	}
	out := &PickList{Nummer: x.Nummer, ProjectNummer: x.ProjectNummer, ProjectName: x.ProjectName,
		PositionNummer: x.PositionNummer, PositionName: x.PositionName, VarianteName: x.VarianteName,
		Stueckzahl: x.Stueckzahl, Liefertermin: x.Liefertermin, Status: x.Status, Lines: make([]PickLine, 0)}
	if x.ReservierungLagerID != nil {
		var code, name string
		if err := s.pg.QueryRow(ctx, `SELECT code, name FROM warehouses WHERE id=$1 AND company_id=$2`, *x.ReservierungLagerID, companyID).
			Scan(&code, &name); err != nil {
			return nil, err
		}
		out.Lager = code + " – " + name
	}

	var materialIDs []string
	for _, it := range x.Items {
		if it.MaterialID != nil {
			materialIDs = append(materialIDs, *it.MaterialID)
		}
	}
	type materialInfo struct{ nummer, bezeichnung string }
	infos := make(map[string]materialInfo)
	places := make(map[string][]stockPlace)
	if len(materialIDs) > 0 {
		rows, err := s.pg.Query(ctx, `SELECT id, nummer, bezeichnung FROM materials WHERE id = ANY($1) AND company_id=$2`, materialIDs, companyID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var mid string
			var m materialInfo
			if err := rows.Scan(&mid, &m.nummer, &m.bezeichnung); err != nil {
				rows.Close()
				return nil, err
			}
			infos[mid] = m
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
		rows, err = s.pg.Query(ctx, `SELECT sm.material_id, w.code, COALESCE(l.code,''), SUM(sm.quantity)::float8
			  FROM stock_movements sm
			  JOIN warehouses w ON w.id = sm.warehouse_id AND w.company_id = $2
			  LEFT JOIN locations l ON l.id = sm.location_id
			 WHERE sm.material_id = ANY($1) AND ($3::text IS NULL OR sm.warehouse_id = $3)
			 GROUP BY sm.material_id, w.code, l.code`, materialIDs, companyID, x.ReservierungLagerID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var mid string
			var p stockPlace
			if err := rows.Scan(&mid, &p.lager, &p.platz, &p.qty); err != nil {
				rows.Close()
				return nil, err
			}
			places[mid] = append(places[mid], p)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	for _, it := range x.Items {
		menge, einheit, gesamt, keep := PickQty(it)
		if !keep {
			continue
		}
		line := PickLine{Pos: it.Pos, Art: it.Art, ArticleCode: it.ArticleCode, Description: it.Description,
			Abmessung: PickAbmessung(it), Menge: menge, Einheit: einheit, Gesamt: gesamt, Plaetze: []string{}}
		if it.MaterialID != nil {
			m := infos[*it.MaterialID]
			line.MaterialNummer, line.MaterialBezeichnung = m.nummer, m.bezeichnung
			line.Plaetze = FormatPickPlaces(places[*it.MaterialID], it.MaterialEinheit, MaxPickPlaces)
		}
		out.Lines = append(out.Lines, line)
	}
	return out, nil
}
