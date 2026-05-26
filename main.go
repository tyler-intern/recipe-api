package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var coll *mongo.Collection

type ingredient struct {
	Item   string `json:"item" bson:"item"`
	Amount string `json:"amount" bson:"amount"`
	Unit   string `json:"unit" bson:"unit"`
}

// DB schema
type recipeDoc struct {
	ID              primitive.ObjectID `bson:"_id"`
	Title           string             `bson:"title"`
	Description     string             `bson:"description,omitempty"`
	PrepTimeMinutes int                `bson:"prepTimeMinutes,omitempty"`
	Servings        int                `bson:"servings,omitempty"`
	Tags            []string           `bson:"tags,omitempty"`
	Notes           string             `bson:"notes,omitempty"`
	Ingredients     []ingredient       `bson:"ingredients"`
	Steps           []string           `bson:"steps"`
	CreatedAt       time.Time          `bson:"createdAt"`
	UpdatedAt       time.Time          `bson:"updatedAt"`
}

// JSON schema
type recipe struct {
	ID              string       `json:"id"`
	Title           string       `json:"title"`
	Description     string       `json:"description,omitempty"`
	PrepTimeMinutes int          `json:"prepTimeMinutes,omitempty"`
	Servings        int          `json:"servings,omitempty"`
	Tags            []string     `json:"tags,omitempty"`
	Notes           string       `json:"notes,omitempty"`
	Ingredients     []ingredient `json:"ingredients"`
	Steps           []string     `json:"steps"`
	CreatedAt       time.Time    `json:"createdAt"`
	UpdatedAt       time.Time    `json:"updatedAt"`
}

// Body for POST /recipes (no id or timestamps from client).
type createRecipeReq struct {
	Title           string       `json:"title"`
	Description     string       `json:"description,omitempty"`
	PrepTimeMinutes int          `json:"prepTimeMinutes,omitempty"`
	Servings        int          `json:"servings,omitempty"`
	Tags            []string     `json:"tags,omitempty"`
	Notes           string       `json:"notes,omitempty"`
	Ingredients     []ingredient `json:"ingredients"`
	Steps           []string     `json:"steps"`
}

// Body for PATCH /recipes/:id (only sent fields are updated).
type patchRecipeReq struct {
	Title           *string       `json:"title"`
	Description     *string       `json:"description"`
	PrepTimeMinutes *int          `json:"prepTimeMinutes"`
	Servings        *int          `json:"servings"`
	Tags            *[]string     `json:"tags"`
	Notes           *string       `json:"notes"`
	Ingredients     *[]ingredient `json:"ingredients"`
	Steps           *[]string     `json:"steps"`
}

// DB to JSON
func docToRecipe(d recipeDoc) recipe {
	return recipe{
		ID:              d.ID.Hex(),
		Title:           d.Title,
		Description:     d.Description,
		PrepTimeMinutes: d.PrepTimeMinutes,
		Servings:        d.Servings,
		Tags:            d.Tags,
		Notes:           d.Notes,
		Ingredients:     d.Ingredients,
		Steps:           d.Steps,
		CreatedAt:       d.CreatedAt,
		UpdatedAt:       d.UpdatedAt,
	}
}

// Write JSON response
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encode json: %v", err)
	}
}

// Parse recipe ID from URL
func parseRecipeID(w http.ResponseWriter, r *http.Request) (primitive.ObjectID, bool) {
	idHex := strings.TrimPrefix(r.URL.Path, "/recipes/")
	idHex = strings.Trim(idHex, "/")
	if idHex == "" {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return primitive.NilObjectID, false
	}
	objectID, err := primitive.ObjectIDFromHex(idHex)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return primitive.NilObjectID, false
	}
	return objectID, true
}

// Validate ingredients (at least one valid ingredient is required)
func validateIngredients(ings []ingredient) bool {
	if len(ings) < 1 {
		return false
	}
	for _, ing := range ings {
		if strings.TrimSpace(ing.Item) == "" ||
			strings.TrimSpace(ing.Amount) == "" ||
			strings.TrimSpace(ing.Unit) == "" {
			return false
		}
	}
	return true
}

// Validate steps (at least one step is required)
func validateSteps(steps []string) bool {
	if len(steps) < 1 {
		return false
	}
	for _, s := range steps {
		if strings.TrimSpace(s) == "" {
			return false
		}
	}
	return true
}

// Uses above two fcns and checks title & preptime are valid
func validateCreate(req createRecipeReq) string {
	if strings.TrimSpace(req.Title) == "" {
		return "title is required"
	}
	if !validateIngredients(req.Ingredients) {
		return "at least one valid ingredient is required"
	}
	if !validateSteps(req.Steps) {
		return "at least one step is required"
	}
	if req.PrepTimeMinutes < 0 || req.Servings < 0 {
		return "prepTimeMinutes and servings must be >= 0"
	}
	return ""
}

// list is get, create is post
func recipesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		listRecipes(w, r)
	case http.MethodPost:
		createRecipe(w, r)
	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// List recipes - get all, can filter by tag
func listRecipes(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	filter := bson.M{}
	if tag := strings.TrimSpace(r.URL.Query().Get("tag")); tag != "" {
		filter["tags"] = tag
	}

	cursor, err := coll.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		log.Printf("find recipes: %v", err)
		http.Error(w, `{"error":"database query failed"}`, http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	docs := []recipeDoc{}
	if err := cursor.All(ctx, &docs); err != nil {
		log.Printf("decode recipes: %v", err)
		http.Error(w, `{"error":"database query failed"}`, http.StatusInternalServerError)
		return
	}

	out := make([]recipe, 0, len(docs))
	for _, d := range docs {
		out = append(out, docToRecipe(d))
	}
	writeJSON(w, http.StatusOK, out)
}

// Create recipe - post. Creates new recipe document in DB
func createRecipe(w http.ResponseWriter, r *http.Request) {
	var req createRecipeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}
	if msg := validateCreate(req); msg != "" {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, msg), http.StatusBadRequest)
		return
	}

	now := time.Now()
	doc := recipeDoc{
		ID:              primitive.NewObjectID(),
		Title:           strings.TrimSpace(req.Title),
		Description:     strings.TrimSpace(req.Description),
		PrepTimeMinutes: req.PrepTimeMinutes,
		Servings:        req.Servings,
		Tags:            req.Tags,
		Notes:           strings.TrimSpace(req.Notes),
		Ingredients:     req.Ingredients,
		Steps:           req.Steps,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if _, err := coll.InsertOne(ctx, doc); err != nil {
		log.Printf("insert recipe: %v", err)
		http.Error(w, `{"error":"database insert failed"}`, http.StatusInternalServerError)
		return
	}
	// Returns the recipe document as JSON
	writeJSON(w, http.StatusCreated, docToRecipe(doc))
}

// Recipe item handler - get, patch, delete
func recipeItemHandler(w http.ResponseWriter, r *http.Request) {
	objectID, ok := parseRecipeID(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		getRecipe(w, r, objectID)
	case http.MethodPatch:
		patchRecipe(w, r, objectID)
	case http.MethodDelete:
		deleteRecipe(w, r, objectID)
	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// Get one recipe - get
func getRecipe(w http.ResponseWriter, r *http.Request, objectID primitive.ObjectID) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var doc recipeDoc
	err := coll.FindOne(ctx, bson.M{"_id": objectID}).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		log.Printf("find recipe: %v", err)
		http.Error(w, `{"error":"database query failed"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, docToRecipe(doc))
}

// Update one recipe - patch
func patchRecipe(w http.ResponseWriter, r *http.Request, objectID primitive.ObjectID) {
	var req patchRecipeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}

	set := bson.M{}
	if req.Title != nil {
		if strings.TrimSpace(*req.Title) == "" {
			http.Error(w, `{"error":"title cannot be empty"}`, http.StatusBadRequest)
			return
		}
		set["title"] = strings.TrimSpace(*req.Title)
	}
	if req.Description != nil {
		set["description"] = strings.TrimSpace(*req.Description)
	}
	if req.PrepTimeMinutes != nil {
		if *req.PrepTimeMinutes < 0 {
			http.Error(w, `{"error":"prepTimeMinutes must be >= 0"}`, http.StatusBadRequest)
			return
		}
		set["prepTimeMinutes"] = *req.PrepTimeMinutes
	}
	if req.Servings != nil {
		if *req.Servings < 0 {
			http.Error(w, `{"error":"servings must be >= 0"}`, http.StatusBadRequest)
			return
		}
		set["servings"] = *req.Servings
	}
	if req.Tags != nil {
		set["tags"] = *req.Tags
	}
	if req.Notes != nil {
		set["notes"] = strings.TrimSpace(*req.Notes)
	}
	if req.Ingredients != nil {
		if !validateIngredients(*req.Ingredients) {
			http.Error(w, `{"error":"at least one valid ingredient is required"}`, http.StatusBadRequest)
			return
		}
		set["ingredients"] = *req.Ingredients
	}
	if req.Steps != nil {
		if !validateSteps(*req.Steps) {
			http.Error(w, `{"error":"at least one step is required"}`, http.StatusBadRequest)
			return
		}
		set["steps"] = *req.Steps
	}
	// If no fields to update, return error
	if len(set) == 0 {
		http.Error(w, `{"error":"no fields to update"}`, http.StatusBadRequest)
		return
	}
	set["updatedAt"] = time.Now()

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	res, err := coll.UpdateOne(ctx, bson.M{"_id": objectID}, bson.M{"$set": set})
	if err != nil {
		log.Printf("update recipe: %v", err)
		http.Error(w, `{"error":"database update failed"}`, http.StatusInternalServerError)
		return
	}
	if res.MatchedCount == 0 {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	var doc recipeDoc
	if err := coll.FindOne(ctx, bson.M{"_id": objectID}).Decode(&doc); err != nil {
		log.Printf("find updated recipe: %v", err)
		http.Error(w, `{"error":"database query failed"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, docToRecipe(doc))
}

func deleteRecipe(w http.ResponseWriter, r *http.Request, objectID primitive.ObjectID) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	res, err := coll.DeleteOne(ctx, bson.M{"_id": objectID})
	if err != nil {
		log.Printf("delete recipe: %v", err)
		http.Error(w, `{"error":"database delete failed"}`, http.StatusInternalServerError)
		return
	}
	if res.DeletedCount == 0 {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Enable CORS, allows requests from the web app
func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3001"
	}

	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		log.Fatal("MONGODB_URI is not set (try: export MONGODB_URI=mongodb://127.0.0.1:27017/recipe_box)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer func() {
		_ = client.Disconnect(context.Background())
	}()

	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("ping: %v", err)
	}
	log.Println("Connected to MongoDB")

	coll = client.Database("recipe_box").Collection("recipes")

	mux := http.NewServeMux()
	mux.HandleFunc("/recipes", recipesHandler)
	mux.HandleFunc("/recipes/", recipeItemHandler)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})

	handler := enableCORS(mux)
	fmt.Printf("API listening on http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, handler))
}
