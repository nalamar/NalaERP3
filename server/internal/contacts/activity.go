package contacts

import (
    "context"
    "fmt"
    "sort"
    "strings"
    "time"
)

type ActivityItem struct {
    ID           string    `json:"id"`
    ContactID    string    `json:"contact_id"`
    Quelle       string    `json:"quelle"`
    Aktion       string    `json:"aktion"`
    ReferenzID   string    `json:"referenz_id"`
    Titel        string    `json:"titel"`
    Beschreibung string    `json:"beschreibung"`
    Zeitpunkt    time.Time `json:"zeitpunkt"`
}

func (s *Service) ListActivity(ctx context.Context, contactID string) ([]ActivityItem, error) {
    c, err := s.Get(ctx, contactID)
    if err != nil {
        return nil, err
    }

    notes, err := s.ListNotes(ctx, contactID)
    if err != nil {
        return nil, err
    }
    tasks, err := s.ListTasks(ctx, contactID)
    if err != nil {
        return nil, err
    }
    docs, err := s.ListContactDocuments(ctx, contactID)
    if err != nil {
        return nil, err
    }

    out := make([]ActivityItem, 0, 1+len(notes)+len(tasks)+len(docs))
    out = append(out, ActivityItem{
        ID:         "contact-created-" + c.ID,
        ContactID:  c.ID,
        Quelle:     "contact",
        Aktion:     "created",
        ReferenzID: c.ID,
        Titel:      "Kontakt angelegt",
        Beschreibung: fmt.Sprintf(
            "%s wurde als %s angelegt.",
            strings.TrimSpace(c.Name),
            c.Rolle,
        ),
        Zeitpunkt: c.Angelegt,
    })

    for _, n := range notes {
        action := "created"
        title := "Notiz angelegt"
        at := n.ErstelltAm
        if !n.AktualisiertAm.Equal(n.ErstelltAm) {
            action = "updated"
            title = "Notiz aktualisiert"
            at = n.AktualisiertAm
        }
        out = append(out, ActivityItem{
            ID:           "note-" + action + "-" + n.ID,
            ContactID:    contactID,
            Quelle:       "note",
            Aktion:       action,
            ReferenzID:   n.ID,
            Titel:        title,
            Beschreibung: summarizeActivityText(firstNonEmpty(n.Titel, n.Inhalt)),
            Zeitpunkt:    at,
        })
    }

    for _, t := range tasks {
        action := "created"
        title := "Aufgabe angelegt"
        at := t.ErstelltAm
        if t.ErledigtAm != nil {
            action = "completed"
            title = "Aufgabe erledigt"
            at = *t.ErledigtAm
        } else if !t.AktualisiertAm.Equal(t.ErstelltAm) {
            action = "updated"
            title = "Aufgabe aktualisiert"
            at = t.AktualisiertAm
        }
        description := strings.TrimSpace(t.Titel)
        if description == "" {
            description = "Aufgabe"
        }
        if strings.TrimSpace(t.Status) != "" {
            description = fmt.Sprintf("%s (%s)", description, t.Status)
        }
        out = append(out, ActivityItem{
            ID:           "task-" + action + "-" + t.ID,
            ContactID:    contactID,
            Quelle:       "task",
            Aktion:       action,
            ReferenzID:   t.ID,
            Titel:        title,
            Beschreibung: summarizeActivityText(description),
            Zeitpunkt:    at,
        })
    }

    for _, d := range docs {
        description := strings.TrimSpace(d.Filename)
        if description == "" {
            description = "Dokument"
        }
        out = append(out, ActivityItem{
            ID:           "document-uploaded-" + d.ID,
            ContactID:    contactID,
            Quelle:       "document",
            Aktion:       "uploaded",
            ReferenzID:   d.DocumentID,
            Titel:        "Dokument hochgeladen",
            Beschreibung: summarizeActivityText(description),
            Zeitpunkt:    d.UploadedAt,
        })
    }

    sort.SliceStable(out, func(i, j int) bool {
        if out[i].Zeitpunkt.Equal(out[j].Zeitpunkt) {
            return out[i].ID > out[j].ID
        }
        return out[i].Zeitpunkt.After(out[j].Zeitpunkt)
    })

    return out, nil
}

func firstNonEmpty(values ...string) string {
    for _, value := range values {
        trimmed := strings.TrimSpace(value)
        if trimmed != "" {
            return trimmed
        }
    }
    return ""
}

func summarizeActivityText(value string) string {
    trimmed := strings.TrimSpace(value)
    if trimmed == "" {
        return ""
    }
    const max = 140
    runes := []rune(trimmed)
    if len(runes) <= max {
        return trimmed
    }
    return string(runes[:max-1]) + "…"
}
