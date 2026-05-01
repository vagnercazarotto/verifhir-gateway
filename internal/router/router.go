package router

import (
	"fmt"

	"github.com/vagnercazarotto/verifhir-gateway/internal/model"
)

// Route logs destination simulation for now.
func Route(payload model.RoutedPayload) {
	fmt.Printf("[router] resource=%s id=%s score=%.2f warnings=%d\n",
		payload.Resource.ResourceType,
		payload.Resource.ID,
		payload.Quality.Score,
		len(payload.Quality.Warnings),
	)
}
