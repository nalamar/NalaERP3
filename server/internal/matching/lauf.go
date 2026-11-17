package matching

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"nalaerp3/internal/llm"
	"nalaerp3/internal/quotes/gaeb"
)

// Lauf ueber einen GAEB-Import (ADR 0045 Fragen 2 und 3, ADR 0046 Frage 2,
// Backlog I.5.2b): erzeugt je Position einen Vorschlag und speichert ihn je
// Buendel, mit Status und Fortschritt am Importlauf. Ein neuer Lauf ersetzt
// nur offene Vorschlaege; entschiedene Positionen gehen nicht erneut an den
// Matcher. Preise gibt es hier nicht.

const (
	QuelleModell       = "modell"
	QuelleWortabgleich = "wortabgleich"

	// SchwelleWortabgleich: fest fuer den Lauf ohne Sprachmodell (ADR 0045 Frage 3).
	SchwelleWortabgleich = 0.7
	// buendelWortabgleich: Positionen je gespeichertem Schritt ohne Modell.
	buendelWortabgleich = 50
	maxBegruendungDB    = 2000
)

// ProfilQuelle liefert den Client eines Modellprofils (llm.SettingsService).
type ProfilQuelle interface {
	ProfileClient(ctx context.Context, companyID, id string, recorder llm.Recorder) (*llm.Client, *llm.ProfilAbschrift, error)
}

// LaufEingabe: Wortabgleich = ohne Sprachmodell; sonst ProfilID oder ""
// fuer das Standardprofil. Ohne Angabe und ohne eingeschaltetes Sprachmodell
// laeuft der Wortabgleich (ADR 0045 Frage 3 (a)); ein ausdruecklich
// gewaehltes Profil muss eingeschaltet sein.
type LaufEingabe struct {
	ProfilID     string `json:"profil_id"`
	Wortabgleich bool   `json:"wortabgleich"`
}

// LaufStatus ist der Stand des Laufs am Importlauf.
type LaufStatus struct {
	Status      string     `json:"status"` // '' / laeuft / fertig / fehler / abgebrochen
	Fehler      string     `json:"fehler"`
	Quelle      string     `json:"quelle"`
	ProfilID    *string    `json:"profil_id"`
	ProfilName  string     `json:"profil_name"`
	Anbieter    string     `json:"anbieter"`
	Modell      string     `json:"modell"`
	Schwelle    *float64   `json:"schwelle"`
	Fortschritt int        `json:"fortschritt"`
	Gesamt      int        `json:"gesamt"`
	GestartetAm *time.Time `json:"gestartet_at"`
	BeendetAm   *time.Time `json:"beendet_at"`
}

// LaufService startet Laeufe und liest ihren Status.
type LaufService struct {
	store   *Store
	profile ProfilQuelle
}

func NewLaufService(store *Store, profile ProfilQuelle) *LaufService {
	return &LaufService{store: store, profile: profile}
}

// ErrLaufAktiv: fuer diesen Importlauf laeuft bereits ein Lauf.
var ErrLaufAktiv = errors.New("Zuordnung läuft bereits")

// Status liefert den Stand des Laufs.
func (s *LaufService) Status(ctx context.Context, importID, companyID string) (*LaufStatus, error) {
	if _, _, err := s.store.importStatus(ctx, s.store.pg.QueryRow, importID, companyID, ""); err != nil {
		return nil, err
	}
	var st LaufStatus
	err := s.store.pg.QueryRow(ctx, `SELECT match_status, match_fehler, match_quelle, match_profil_id::text, match_profil_name, match_anbieter,
		match_modell, match_schwelle::float8, match_fortschritt, match_gesamt, match_gestartet_at, match_beendet_at
		FROM quote_imports WHERE id=$1`, importID).Scan(&st.Status, &st.Fehler, &st.Quelle, &st.ProfilID, &st.ProfilName, &st.Anbieter,
		&st.Modell, &st.Schwelle, &st.Fortschritt, &st.Gesamt, &st.GestartetAm, &st.BeendetAm)
	return &st, err
}

// Starten prueft und belegt den Importlauf (Status "laeuft") und liefert
// die Arbeit als Funktion: der Aufrufer fuehrt sie aus, im Betrieb in einer
// Goroutine mit eigenem Context (I.5.3). Sie schreibt Ergebnis und Status
// selbst und gibt den Fehler des Laufs nur zur Information zurueck.
func (s *LaufService) Starten(ctx context.Context, importID string, in LaufEingabe, actorUserID, companyID string) (*LaufStatus, func(context.Context) error, error) {
	if _, status, err := s.store.importStatus(ctx, s.store.pg.QueryRow, importID, companyID, ""); err != nil {
		return nil, nil, err
	} else if status != "parsed" {
		return nil, nil, errors.New("Zuordnung im aktuellen Status nicht erlaubt (nur verarbeitete, noch nicht freigegebene Importläufe)")
	}
	lv, positionen, err := s.store.LadeLV(ctx, importID, companyID)
	if err != nil {
		return nil, nil, err
	}
	k, ids, err := s.store.LadeKatalog(ctx, companyID)
	if err != nil {
		return nil, nil, err
	}
	if len(k.Leistungen) == 0 {
		return nil, nil, errors.New("Leistungskatalog ist leer: es fehlt mindestens eine aktive Leistung")
	}
	matcher, buendel, ab, err := s.waehle(ctx, in, companyID)
	if err != nil {
		return nil, nil, err
	}
	quelle, schwelle := QuelleWortabgleich, SchwelleWortabgleich
	var profilID *string
	if ab != nil {
		quelle, schwelle, profilID = QuelleModell, ab.Schwelle, &ab.ProfilID
	} else {
		ab = &llm.ProfilAbschrift{}
	}

	// nur Positionen ohne Entscheidung gehen an den Matcher
	offen, err := s.store.unentschieden(ctx, importID, lv, positionen)
	if err != nil {
		return nil, nil, err
	}
	var actor *string
	if strings.TrimSpace(actorUserID) != "" {
		actor = &actorUserID
	}
	tag, err := s.store.pg.Exec(ctx, `
		UPDATE quote_imports SET match_status='laeuft', match_fehler='', match_quelle=$2, match_profil_id=$3, match_profil_name=$4,
			match_anbieter=$5, match_modell=$6, match_schwelle=$7, match_fortschritt=0, match_gesamt=$8,
			match_gestartet_von_user_id=$9, match_gestartet_at=now(), match_beendet_at=NULL
		WHERE id=$1 AND match_status<>'laeuft' AND status='parsed'`,
		importID, quelle, profilID, ab.Name, ab.Anbieter, ab.Modell, schwelle, len(offen.Positionen), actor)
	if err != nil {
		return nil, nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, nil, ErrLaufAktiv
	}
	st, err := s.Status(ctx, importID, companyID)
	if err != nil {
		return nil, nil, err
	}
	z := zeile{importID: importID, quelle: quelle, profilID: profilID, ab: *ab, schwelle: schwelle}
	run := func(ctx context.Context) error {
		err := s.ausfuehren(ctx, matcher, buendel, offen, k, positionen, ids, z)
		status, meldung := "fertig", ""
		if err != nil {
			status, meldung = "fehler", kuerzen(err.Error(), maxBegruendungDB-2)
		}
		// eigener Context: auch nach Abbruch den Status schreiben
		wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		if _, uerr := s.store.pg.Exec(wctx, `UPDATE quote_imports SET match_status=$2, match_fehler=$3, match_beendet_at=now()
			WHERE id=$1 AND match_status='laeuft'`, importID, status, meldung); uerr != nil && err == nil {
			err = uerr
		}
		return err
	}
	return st, run, nil
}

// waehle bestimmt Matcher, Buendelgroesse und (mit Modell) die Abschrift.
func (s *LaufService) waehle(ctx context.Context, in LaufEingabe, companyID string) (LVMatcher, int, *llm.ProfilAbschrift, error) {
	wort := MitSystemvorgaben(JePosition(Wortabgleich{}))
	if in.Wortabgleich {
		if in.ProfilID != "" {
			return nil, 0, nil, errors.New("Auswahl ungültig: entweder ein Profil oder den Wortabgleich")
		}
		return wort, buendelWortabgleich, nil, nil
	}
	if s.profile == nil {
		return nil, 0, nil, errors.New("Sprachmodell nicht eingerichtet")
	}
	c, ab, err := s.profile.ProfileClient(ctx, companyID, in.ProfilID, nil)
	if in.ProfilID == "" && (errors.Is(err, llm.ErrNotEnabled) || errors.Is(err, llm.ErrProfileNotFound)) {
		return wort, buendelWortabgleich, nil, nil
	}
	if err != nil {
		return nil, 0, nil, err
	}
	m := ModellMatcher{Client: c, Modell: ab.Modell}
	_, _, n := m.groessen()
	return MitSystemvorgaben(m), n, ab, nil
}

// zeile: was jeder Vorschlag eines Laufs als Abschrift traegt.
type zeile struct {
	importID string
	quelle   string
	profilID *string
	ab       llm.ProfilAbschrift
	schwelle float64
}

// ausfuehren arbeitet die Buendel ab (wie ModellMatcher buendelt: nach
// Bereich, hoechstens n Positionen) und speichert nach jedem Buendel.
func (s *LaufService) ausfuehren(ctx context.Context, m LVMatcher, n int, lv *gaeb.LV, k *Katalog, positionen, ids map[string]string, z zeile) error {
	fertig := 0
	for _, b := range buendeln(lv.Positionen, n) {
		teil := &gaeb.LV{Vorbemerkungen: lv.Vorbemerkungen, Bereiche: lv.Bereiche, Positionen: b}
		vs, err := m.VorschlagenLV(ctx, teil, k)
		if err != nil {
			return err
		}
		if err := s.store.speichern(ctx, b, vs, positionen, ids, z); err != nil {
			return err
		}
		fertig += len(b)
		if _, err := s.store.pg.Exec(ctx, `UPDATE quote_imports SET match_fortschritt=$2 WHERE id=$1`, z.importID, fertig); err != nil {
			return err
		}
	}
	return nil
}

// MarkiereAbgebrochen setzt Laeufe, die beim Start des Prozesses noch als
// "laeuft" stehen, auf "abgebrochen" (der Prozess endete waehrend des
// Laufs); sie koennen neu gestartet werden.
func (s *LaufService) MarkiereAbgebrochen(ctx context.Context) (int64, error) {
	tag, err := s.store.pg.Exec(ctx, `UPDATE quote_imports SET match_status='abgebrochen',
		match_fehler='Lauf durch Neustart des Servers abgebrochen', match_beendet_at=now() WHERE match_status='laeuft'`)
	return tag.RowsAffected(), err
}

// importStatus prueft den Importlauf beim Mandanten und liefert ID und Status.
func (s *Store) importStatus(ctx context.Context, q func(context.Context, string, ...any) pgx.Row, importID, companyID, suffix string) (string, string, error) {
	if strings.TrimSpace(companyID) == "" {
		return "", "", errors.New("Mandant erforderlich")
	}
	if !istUUID(importID) {
		return "", "", ErrImportNotFound
	}
	var id, status string
	err := q(ctx, `SELECT i.id::text, i.status FROM quote_imports i JOIN projects p ON p.id = i.project_id
		WHERE i.id=$1 AND i.source_kind='gaeb' AND p.company_id=$2`+suffix, importID, companyID).Scan(&id, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", ErrImportNotFound
	}
	return id, status, err
}

// unentschieden liefert das LV ohne Positionen mit entschiedenem Vorschlag.
func (s *Store) unentschieden(ctx context.Context, importID string, lv *gaeb.LV, positionen map[string]string) (*gaeb.LV, error) {
	rows, err := s.pg.Query(ctx, `SELECT quote_import_item_id::text FROM quote_import_item_matches
		WHERE import_id=$1 AND entscheidung<>'offen'`, importID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entschieden := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		entschieden[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := *lv
	out.Positionen = nil
	for _, p := range lv.Positionen {
		if !entschieden[positionen[p.OZ]] {
			out.Positionen = append(out.Positionen, p)
		}
	}
	return &out, nil
}

// speichern schreibt die Vorschlaege eines Buendels. Entschiedene bleiben,
// auch wenn die Entscheidung waehrend des Laufs fiel. Score ausserhalb 0-1
// lehnt die Datenbank ab (die Matcher liefern keinen).
func (s *Store) speichern(ctx context.Context, b []gaeb.Position, vs map[string]Vorschlag, positionen, ids map[string]string, z zeile) error {
	tx, err := s.pg.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for _, p := range b {
		v := vs[p.OZ]
		var leistung *string
		if id, ok := ids[v.Code]; ok {
			leistung = &id
		}
		sv, err := json.Marshal(v.Systemvorgaben)
		if err != nil {
			return err
		}
		// Schutz fuer die Pruefung der Datenbank; die Matcher kuerzen selbst.
		begruendung := kuerzen(v.Begruendung, maxBegruendungDB-2)
		if _, err := tx.Exec(ctx, `
			INSERT INTO quote_import_item_matches (quote_import_item_id, import_id, leistung_id, code, score, begruendung, systemvorgaben,
				quelle, profil_id, profil_name, anbieter, modell, schwelle, vorgeschlagen_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,now())
			ON CONFLICT (quote_import_item_id) DO UPDATE SET
				leistung_id=EXCLUDED.leistung_id, code=EXCLUDED.code, score=EXCLUDED.score, begruendung=EXCLUDED.begruendung,
				systemvorgaben=EXCLUDED.systemvorgaben, quelle=EXCLUDED.quelle, profil_id=EXCLUDED.profil_id,
				profil_name=EXCLUDED.profil_name, anbieter=EXCLUDED.anbieter, modell=EXCLUDED.modell, schwelle=EXCLUDED.schwelle,
				vorgeschlagen_at=now()
			WHERE quote_import_item_matches.entscheidung='offen'`,
			positionen[p.OZ], z.importID, leistung, v.Code, v.Score, begruendung, sv,
			z.quelle, z.profilID, z.ab.Name, z.ab.Anbieter, z.ab.Modell, z.schwelle); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func istUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}
