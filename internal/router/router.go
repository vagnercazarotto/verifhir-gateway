package router

import (
	"fmt"

	"github.com/vagnercazarotto/verifhir-gateway/internal/model"
)

// Route dispatches a routed payload to its destination.
// Currently a stdout stub; will be replaced by HTTP / queue adapters in Phase 2.
func Route(payload model.RoutedPayload) {
	fmt.Printf("[router] resource=%s id=%s score=%.2f findings=%d\n",
		payload.Resource.ResourceType,
		payload.Resource.ID,
		payload.Quality.Score,
		len(payload.Quality.Findings),
	)
}
