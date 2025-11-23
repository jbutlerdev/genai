package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"
	_ "github.com/lib/pq"
)

// EmbeddingProvider defines the interface for generating embeddings
type EmbeddingProvider interface {
	// GenerateEmbedding generates an embedding for a single text input
	GenerateEmbedding(ctx context.Context, text string, model string) ([]float32, error)

	// GenerateEmbeddings generates embeddings for multiple text inputs
	GenerateEmbeddings(ctx context.Context, texts []string, model string) ([][]float32, error)
}

// MemoryEntry represents a stored memory with its metadata
type MemoryEntry struct {
	ID        string                 `json:"id"`
	Content   string                 `json:"content"`
	Metadata  map[string]interface{} `json:"metadata"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	ExpiresAt *time.Time             `json:"expires_at,omitempty"`
}

// MemoryResult represents a retrieved memory with similarity score
type MemoryResult struct {
	MemoryEntry
	Similarity float64 `json:"similarity"`
}

// RetrieveOptions configures how memories are retrieved
type RetrieveOptions struct {
	TopK    int                    `json:"top_k"`
	Filters map[string]interface{} `json:"filters,omitempty"`
}

// MemoryConfig holds configuration for the MemoryTool
type MemoryConfig struct {
	DatabaseURL       string
	EmbeddingProvider string
	EmbeddingModel    string
	EmbeddingDims     int
	DefaultTTL        time.Duration
	DefaultTopK       int
}

// MemoryTool implements the core memory functionality
type MemoryTool struct {
	db     *sql.DB
	config MemoryConfig
	embeddingProvider EmbeddingProvider
}

// NewMemoryTool creates a new MemoryTool instance
func NewMemoryTool(config MemoryConfig, embeddingProvider EmbeddingProvider) (*MemoryTool, error) {
	db, err := sql.Open("postgres", config.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Initialize database schema
	if err := initSchema(db); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return &MemoryTool{
		db:     db,
		config: config,
		embeddingProvider: embeddingProvider,
	}, nil
}

// initSchema creates the necessary database tables and indexes
func initSchema(db *sql.DB) error {
	// Try to create the vector extension, but don't fail if we can't
	_, extErr := db.Exec("CREATE EXTENSION IF NOT EXISTS vector")
	if extErr != nil {
		// Log the error but continue - we might be able to work without it for testing
		fmt.Printf("Warning: Could not create vector extension: %v\n", extErr)
	}

	// Use a fixed dimension for the vector type. In PostgreSQL, table schema definitions
	// cannot use parameters, so we need to specify the dimension directly.
	// We'll use 1536 as the default dimension which matches common embedding models.
	schema := `
	CREATE TABLE IF NOT EXISTS memories (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		content TEXT NOT NULL,
		embedding VECTOR(1536),
		metadata JSONB,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		expires_at TIMESTAMP WITH TIME ZONE
	);
	`

	_, err := db.Exec(schema)
	if err != nil {
		return err
	}

	// Try to create indexes, but don't fail if we can't
	indexQueries := []string{
		"CREATE INDEX IF NOT EXISTS idx_memories_expires_at ON memories (expires_at) WHERE expires_at IS NOT NULL",
		"CREATE INDEX IF NOT EXISTS idx_memories_metadata ON memories USING GIN (metadata)",
	}

	// Only try to create vector index if extension is available
	if extErr == nil {
		indexQueries = append([]string{
			"CREATE INDEX IF NOT EXISTS idx_memories_embedding ON memories USING hnsw (embedding vector_cosine_ops)",
		}, indexQueries...)
	}

	for _, query := range indexQueries {
		if _, err := db.Exec(query); err != nil {
			fmt.Printf("Warning: Could not create index with query '%s': %v\n", query, err)
		}
	}

	return nil
}

// generateEmbedding generates vector embeddings for text content using the configured embedding provider
func (mt *MemoryTool) generateEmbedding(ctx context.Context, text string) ([]float32, error) {
	// Use the actual embedding provider to generate embeddings
	embedding, err := mt.embeddingProvider.GenerateEmbedding(ctx, text, mt.config.EmbeddingModel)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Ensure the embedding has the correct dimensions for our table schema
	// Our table schema uses 1536 dimensions, so we need to pad or truncate if necessary
	targetDims := 1536
	
	if len(embedding) > targetDims {
		// Truncate to target dimensions
		embedding = embedding[:targetDims]
	} else if len(embedding) < targetDims {
		// Pad with zeros to reach target dimensions
		padded := make([]float32, targetDims)
		copy(padded, embedding)
		embedding = padded
	}

	return embedding, nil
}

// Store saves a memory with content and metadata
func (mt *MemoryTool) Store(ctx context.Context, content string, metadata map[string]interface{}) (string, error) {
	id := uuid.New().String()

	// Generate embedding for the content
	embedding, err := mt.generateEmbedding(ctx, content)
	if err != nil {
		return "", fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Set expiration time if TTL is configured
	var expiresAt *time.Time
	if mt.config.DefaultTTL > 0 {
		exp := time.Now().Add(mt.config.DefaultTTL)
		expiresAt = &exp
	}

	// Convert metadata to json.RawMessage for proper JSONB handling
	var rawMetadata json.RawMessage
	if metadata != nil {
		jsonData, err := json.Marshal(metadata)
		if err != nil {
			return "", fmt.Errorf("failed to marshal metadata: %w", err)
		}
		rawMetadata = json.RawMessage(jsonData)
	}

	// Insert into database
	query := `
		INSERT INTO memories (id, content, embedding, metadata, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`

	_, err = mt.db.ExecContext(ctx, query, id, content, pgvector.NewVector(embedding), rawMetadata, expiresAt)
	if err != nil {
		return "", fmt.Errorf("failed to store memory: %w", err)
	}

	return id, nil
}

// Retrieve performs semantic search for memories
func (mt *MemoryTool) Retrieve(ctx context.Context, queryText string, options RetrieveOptions) ([]*MemoryResult, error) {
	// Generate embedding for the query
	queryEmbedding, err := mt.generateEmbedding(ctx, queryText)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Set default topK if not specified
	topK := options.TopK
	if topK <= 0 {
		topK = mt.config.DefaultTopK
	}
	if topK <= 0 {
		topK = 5 // fallback default
	}

	// Build query with filters
	baseQuery := `
		SELECT id, content, metadata, created_at, updated_at, expires_at,
		       1 - (embedding <=> $1) as similarity
		FROM memories
		WHERE expires_at IS NULL OR expires_at > NOW()
	`

	args := []interface{}{pgvector.NewVector(queryEmbedding)}
	argIndex := 2

	// Add metadata filters if provided
	if options.Filters != nil && len(options.Filters) > 0 {
		// Convert the entire filter map to JSON
		filterJSON, err := json.Marshal(options.Filters)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal filters: %w", err)
		}
		
		baseQuery += fmt.Sprintf(" AND metadata @> $%d::jsonb", argIndex)
		args = append(args, string(filterJSON))
		argIndex++
	}

	baseQuery += fmt.Sprintf(" ORDER BY embedding <=> $1 LIMIT $%d", argIndex)
	args = append(args, topK)

	rows, err := mt.db.QueryContext(ctx, baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve memories: %w", err)
	}
	defer rows.Close()

	var results []*MemoryResult
	for rows.Next() {
		var mem MemoryResult
		var similarity sql.NullFloat64
		var metadataBytes []byte

		err := rows.Scan(
			&mem.ID,
			&mem.Content,
			&metadataBytes,
			&mem.CreatedAt,
			&mem.UpdatedAt,
			&mem.ExpiresAt,
			&similarity,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan memory: %w", err)
		}

		// Unmarshal metadata from bytes to map
		if metadataBytes != nil {
			err = json.Unmarshal(metadataBytes, &mem.Metadata)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		} else {
			mem.Metadata = make(map[string]interface{})
		}

		mem.Similarity = similarity.Float64
		results = append(results, &mem)
	}

	return results, nil
}

// Update modifies an existing memory entry
func (mt *MemoryTool) Update(ctx context.Context, id string, content string, metadata map[string]interface{}) error {
	// Generate new embedding for updated content
	embedding, err := mt.generateEmbedding(ctx, content)
	if err != nil {
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Convert metadata to json.RawMessage for proper JSONB handling
	var rawMetadata json.RawMessage
	if metadata != nil {
		jsonData, err := json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
		rawMetadata = json.RawMessage(jsonData)
	}

	query := `
		UPDATE memories
		SET content = $1, embedding = $2, metadata = $3, updated_at = NOW()
		WHERE id = $4
	`

	_, err = mt.db.ExecContext(ctx, query, content, pgvector.NewVector(embedding), rawMetadata, id)
	if err != nil {
		return fmt.Errorf("failed to update memory: %w", err)
	}

	return nil
}

// Delete removes a memory entry by ID
func (mt *MemoryTool) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM memories WHERE id = $1`

	_, err := mt.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete memory: %w", err)
	}

	return nil
}

// Close closes the database connection
func (mt *MemoryTool) Close() error {
	return mt.db.Close()
}

// Memory tool constants
const (
	MemoryStoreToolName    = "memory_store"
	MemoryRetrieveToolName = "memory_retrieve"
	MemoryUpdateToolName   = "memory_update"
	MemoryDeleteToolName   = "memory_delete"
)

var memoryTools = map[string]Tool{
	MemoryStoreToolName: {
		Name:        MemoryStoreToolName,
		Description: "Store a memory with content and optional metadata",
		Parameters: []Parameter{
			{Name: "content", Type: "string", Description: "The content to store", Required: true},
			{Name: "metadata", Type: "object", Description: "Optional metadata associated with the memory", Required: false},
		},
		Options: map[string]string{},
		Run: runMemoryStore,
	},
	MemoryRetrieveToolName: {
		Name:        MemoryRetrieveToolName,
		Description: "Retrieve memories based on semantic similarity to a query",
		Parameters: []Parameter{
			{Name: "query", Type: "string", Description: "The query to search for similar memories", Required: true},
			{Name: "top_k", Type: "integer", Description: "Number of results to return", Required: false},
			{Name: "filters", Type: "object", Description: "Metadata filters to apply", Required: false},
		},
		Options: map[string]string{},
		Run: runMemoryRetrieve,
	},
	MemoryUpdateToolName: {
		Name:        MemoryUpdateToolName,
		Description: "Update an existing memory by ID",
		Parameters: []Parameter{
			{Name: "id", Type: "string", Description: "The ID of the memory to update", Required: true},
			{Name: "content", Type: "string", Description: "The new content", Required: true},
			{Name: "metadata", Type: "object", Description: "Optional new metadata", Required: false},
		},
		Options: map[string]string{},
		Run: runMemoryUpdate,
	},
	MemoryDeleteToolName: {
		Name:        MemoryDeleteToolName,
		Description: "Delete a memory by ID",
		Parameters: []Parameter{
			{Name: "id", Type: "string", Description: "The ID of the memory to delete", Required: true},
		},
		Options: map[string]string{},
		Run: runMemoryDelete,
	},
	"memory_operation": {
		Name:        "memory_operation",
		Description: "Perform memory operations (store, retrieve, update, delete)",
		Parameters: []Parameter{
			{Name: "operation", Type: "string", Description: "The operation to perform (store, retrieve, update, delete)", Required: true},
			{Name: "arguments", Type: "object", Description: "Operation-specific arguments", Required: true},
		},
		Options: map[string]string{},
		Run: runMemoryOperation,
	},
}

// MemoryStoreArgs represents arguments for storing a memory
type MemoryStoreArgs struct {
	Content  string                 `json:"content"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// MemoryRetrieveArgs represents arguments for retrieving memories
type MemoryRetrieveArgs struct {
	Query   string                 `json:"query"`
	TopK    int                    `json:"top_k,omitempty"`
	Filters map[string]interface{} `json:"filters,omitempty"`
}

// MemoryUpdateArgs represents arguments for updating a memory
type MemoryUpdateArgs struct {
	ID       string                 `json:"id"`
	Content  string                 `json:"content"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// MemoryDeleteArgs represents arguments for deleting a memory
type MemoryDeleteArgs struct {
	ID string `json:"id"`
}

// Global memory tool instance - in practice this would be initialized properly
var globalMemoryTool *MemoryTool

// InitializeMemoryTool initializes the global memory tool instance
func InitializeMemoryTool(config MemoryConfig, embeddingProvider EmbeddingProvider) error {
	mt, err := NewMemoryTool(config, embeddingProvider)
	if err != nil {
		return err
	}
	globalMemoryTool = mt
	return nil
}

// runMemoryStore handles the memory store operation
func runMemoryStore(args map[string]any) (map[string]any, error) {
	if globalMemoryTool == nil {
		return nil, fmt.Errorf("memory tool not initialized")
	}

	// Parse arguments
	content, ok := args["content"].(string)
	if !ok {
		return nil, fmt.Errorf("content is required and must be a string")
	}

	var metadata map[string]interface{}
	if meta, ok := args["metadata"]; ok {
		if metaMap, ok := meta.(map[string]any); ok {
			metadata = make(map[string]interface{})
			for k, v := range metaMap {
				metadata[k] = v
			}
		}
	}

	// Store the memory
	id, err := globalMemoryTool.Store(context.Background(), content, metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to store memory: %w", err)
	}

	return map[string]any{
		"id": id,
	}, nil
}

// runMemoryRetrieve handles the memory retrieve operation
func runMemoryRetrieve(args map[string]any) (map[string]any, error) {
	if globalMemoryTool == nil {
		return nil, fmt.Errorf("memory tool not initialized")
	}

	// Parse arguments
	query, ok := args["query"].(string)
	if !ok {
		return nil, fmt.Errorf("query is required and must be a string")
	}

	var options RetrieveOptions
	
	if topK, ok := args["top_k"]; ok {
		if topKInt, ok := topK.(int); ok {
			options.TopK = topKInt
		} else if topKFloat, ok := topK.(float64); ok {
			options.TopK = int(topKFloat)
		}
	}

	if filters, ok := args["filters"]; ok {
		if filterMap, ok := filters.(map[string]any); ok {
			options.Filters = make(map[string]interface{})
			for k, v := range filterMap {
				options.Filters[k] = v
			}
		}
	}

	// Retrieve memories
	results, err := globalMemoryTool.Retrieve(context.Background(), query, options)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve memories: %w", err)
	}

	// Convert results to serializable format
	serializableResults := make([]map[string]any, len(results))
	for i, result := range results {
		serializableResults[i] = map[string]any{
			"id":         result.ID,
			"content":    result.Content,
			"metadata":   result.Metadata,
			"similarity": result.Similarity,
			"created_at": result.CreatedAt,
		}
		if result.ExpiresAt != nil {
			serializableResults[i]["expires_at"] = *result.ExpiresAt
		}
	}

	return map[string]any{
		"results": serializableResults,
	}, nil
}

// runMemoryUpdate handles the memory update operation
func runMemoryUpdate(args map[string]any) (map[string]any, error) {
	if globalMemoryTool == nil {
		return nil, fmt.Errorf("memory tool not initialized")
	}

	// Parse arguments
	id, ok := args["id"].(string)
	if !ok {
		return nil, fmt.Errorf("id is required and must be a string")
	}

	content, ok := args["content"].(string)
	if !ok {
		return nil, fmt.Errorf("content is required and must be a string")
	}

	var metadata map[string]interface{}
	if meta, ok := args["metadata"]; ok {
		if metaMap, ok := meta.(map[string]any); ok {
			metadata = make(map[string]interface{})
			for k, v := range metaMap {
				metadata[k] = v
			}
		}
	}

	// Update the memory
	err := globalMemoryTool.Update(context.Background(), id, content, metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to update memory: %w", err)
	}

	return map[string]any{
		"success": true,
	}, nil
}

// runMemoryDelete handles the memory delete operation
func runMemoryDelete(args map[string]any) (map[string]any, error) {
	if globalMemoryTool == nil {
		return nil, fmt.Errorf("memory tool not initialized")
	}

	// Parse arguments
	id, ok := args["id"].(string)
	if !ok {
		return nil, fmt.Errorf("id is required and must be a string")
	}

	// Delete the memory
	err := globalMemoryTool.Delete(context.Background(), id)
	if err != nil {
		return nil, fmt.Errorf("failed to delete memory: %w", err)
	}

	return map[string]any{
		"success": true,
	}, nil
}

// Alternative approach: Single tool with operation parameter
var memoryOperationTool = Tool{
	Name:        "memory_operation",
	Description: "Perform memory operations (store, retrieve, update, delete)",
	Parameters: []Parameter{
		{Name: "operation", Type: "string", Description: "The operation to perform (store, retrieve, update, delete)", Required: true},
		{Name: "arguments", Type: "object", Description: "Operation-specific arguments", Required: true},
	},
	Run: runMemoryOperation,
}

// runMemoryOperation handles all memory operations through a single tool
func runMemoryOperation(args map[string]any) (map[string]any, error) {
	operation, ok := args["operation"].(string)
	if !ok {
		return nil, fmt.Errorf("operation is required and must be a string")
	}

	arguments, ok := args["arguments"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("arguments is required and must be an object")
	}

	switch operation {
	case "store":
		return runMemoryStore(arguments)
	case "retrieve":
		return runMemoryRetrieve(arguments)
	case "update":
		return runMemoryUpdate(arguments)
	case "delete":
		return runMemoryDelete(arguments)
	default:
		return nil, fmt.Errorf("unknown operation: %s", operation)
	}
}
