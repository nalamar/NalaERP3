package materials

import (
    "encoding/json"
)

// toJSONB hilft beim Einfügen von Maps nach jsonb in Postgres
// Wir geben einen JSON-String zurück, der in SQL mittels ::jsonb gecastet wird.
func toJSONB(v map[string]any) string {
    b, _ := json.Marshal(v)
    return string(b)
}
