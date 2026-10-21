package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/pqhuy98/wow-converter/internal/wow/casc"
)

func (c *HTTPClient) ResolveNpcSelection(ctx context.Context, id int, options casc.NpcSelectionOptions) (float64, error) {
	return resolveNpcSelection(ctx, id, options, c.getJSONAllowError)
}

func (c *InProcessClient) ResolveNpcSelection(ctx context.Context, id int, options casc.NpcSelectionOptions) (float64, error) {
	return resolveNpcSelection(ctx, id, options, c.doGet)
}

func resolveNpcSelection(ctx context.Context, id int, o casc.NpcSelectionOptions, get func(context.Context, string, url.Values) (map[string]any, int, error)) (float64, error) {
	values := url.Values{"displayId": {strconv.Itoa(id)}, "selection": {"1"}, "family": {strconv.Itoa(o.Family)}, "level": {strconv.Itoa(o.Level)}, "objectScale": {strconv.FormatFloat(o.ObjectScale, 'g', -1, 64)}}
	raw, status, err := get(ctx, "/rest/resolveNpcDisplay", values)
	if err != nil {
		return 0, err
	}
	if status != http.StatusOK || raw["id"] != "NPC_DISPLAY_META" {
		return 0, fmt.Errorf("resolve NPC selection %d: status %d: %v", id, status, raw["message"])
	}
	meta, err := decodeObject[casc.NpcDisplayMeta](raw)
	if err != nil {
		return 0, err
	}
	if !meta.Found {
		return 1.2, nil
	} // Native GetStandBox returns an empty box.
	if meta.SelectionRadius == nil {
		return 0, fmt.Errorf("backend did not provide native NPC selection metadata for %d", id)
	}
	return *meta.SelectionRadius, nil
}
