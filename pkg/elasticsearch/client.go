package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

type Client struct {
	es *elasticsearch.Client
}

type Config struct {
	Addresses []string `yaml:"addresses"`
	Username  string   `yaml:"username"`
	Password  string   `yaml:"password"`
}

// NewClient creates a new Elasticsearch client
func NewClient(config Config) (*Client, error) {
	cfg := elasticsearch.Config{
		Addresses: config.Addresses,
		Username:  config.Username,
		Password:  config.Password,
	}

	es, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create elasticsearch client: %w", err)
	}

	// Test connection
	res, err := es.Info()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to elasticsearch: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("elasticsearch connection error: %s", res.String())
	}

	log.Println("Successfully connected to Elasticsearch")

	return &Client{es: es}, nil
}

// CreateIndex creates an index with the given mapping
func (c *Client) CreateIndex(ctx context.Context, indexName string, mapping map[string]interface{}) error {
	// Check if index already exists
	req := esapi.IndicesExistsRequest{
		Index: []string{indexName},
	}

	res, err := req.Do(ctx, c.es)
	if err != nil {
		return fmt.Errorf("failed to check index existence: %w", err)
	}
	defer res.Body.Close()

	// If index exists, delete it first (for development)
	if res.StatusCode == 200 {
		log.Printf("Index %s already exists, deleting...", indexName)
		if err := c.DeleteIndex(ctx, indexName); err != nil {
			return fmt.Errorf("failed to delete existing index: %w", err)
		}
	}

	// Create index with mapping
	mappingJSON, err := json.Marshal(mapping)
	if err != nil {
		return fmt.Errorf("failed to marshal mapping: %w", err)
	}

	createReq := esapi.IndicesCreateRequest{
		Index: indexName,
		Body:  bytes.NewReader(mappingJSON),
	}

	createRes, err := createReq.Do(ctx, c.es)
	if err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}
	defer createRes.Body.Close()

	if createRes.IsError() {
		return fmt.Errorf("index creation error: %s", createRes.String())
	}

	log.Printf("Successfully created index: %s", indexName)
	return nil
}

// DeleteIndex deletes an index
func (c *Client) DeleteIndex(ctx context.Context, indexName string) error {
	req := esapi.IndicesDeleteRequest{
		Index: []string{indexName},
	}

	res, err := req.Do(ctx, c.es)
	if err != nil {
		return fmt.Errorf("failed to delete index: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() && res.StatusCode != 404 {
		return fmt.Errorf("index deletion error: %s", res.String())
	}

	return nil
}

// IndexDocument indexes a document
func (c *Client) IndexDocument(ctx context.Context, indexName, documentID string, document interface{}) error {
	docJSON, err := json.Marshal(document)
	if err != nil {
		return fmt.Errorf("failed to marshal document: %w", err)
	}

	req := esapi.IndexRequest{
		Index:      indexName,
		DocumentID: documentID,
		Body:       bytes.NewReader(docJSON),
		Refresh:    "true", // For immediate availability
	}

	res, err := req.Do(ctx, c.es)
	if err != nil {
		return fmt.Errorf("failed to index document: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("document indexing error: %s", res.String())
	}

	return nil
}

// UpdateDocument updates a document
func (c *Client) UpdateDocument(ctx context.Context, indexName, documentID string, document interface{}) error {
	updateDoc := map[string]interface{}{
		"doc": document,
	}

	docJSON, err := json.Marshal(updateDoc)
	if err != nil {
		return fmt.Errorf("failed to marshal update document: %w", err)
	}

	req := esapi.UpdateRequest{
		Index:      indexName,
		DocumentID: documentID,
		Body:       bytes.NewReader(docJSON),
		Refresh:    "true",
	}

	res, err := req.Do(ctx, c.es)
	if err != nil {
		return fmt.Errorf("failed to update document: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("document update error: %s", res.String())
	}

	return nil
}

// BulkIndex performs bulk indexing operations
func (c *Client) BulkIndex(ctx context.Context, operations []BulkOperation) error {
	if len(operations) == 0 {
		return nil
	}

	var buf bytes.Buffer
	for _, op := range operations {
		// Action metadata
		actionMeta := map[string]interface{}{
			op.Action: map[string]interface{}{
				"_index": op.Index,
				"_id":    op.DocumentID,
			},
		}

		actionJSON, err := json.Marshal(actionMeta)
		if err != nil {
			return fmt.Errorf("failed to marshal action metadata: %w", err)
		}

		buf.Write(actionJSON)
		buf.WriteByte('\n')

		// Document body (for index/create operations)
		if op.Action == "index" || op.Action == "create" {
			docJSON, err := json.Marshal(op.Document)
			if err != nil {
				return fmt.Errorf("failed to marshal document: %w", err)
			}

			buf.Write(docJSON)
			buf.WriteByte('\n')
		}
	}

	req := esapi.BulkRequest{
		Body:    strings.NewReader(buf.String()),
		Refresh: "true",
	}

	res, err := req.Do(ctx, c.es)
	if err != nil {
		return fmt.Errorf("failed to perform bulk operation: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("bulk operation error: %s", res.String())
	}

	// Parse response to check for individual operation errors
	var bulkResponse BulkResponse
	if err := json.NewDecoder(res.Body).Decode(&bulkResponse); err != nil {
		return fmt.Errorf("failed to decode bulk response: %w", err)
	}

	if bulkResponse.Errors {
		// Log individual errors but don't fail the entire operation
		for _, item := range bulkResponse.Items {
			for action, details := range item {
				if details.Error != nil {
					log.Printf("Bulk operation error for %s %s: %v", action, details.ID, details.Error)
				}
			}
		}
	}

	return nil
}

// Search performs a search query
func (c *Client) Search(ctx context.Context, indexName string, query map[string]interface{}) (*SearchResponse, error) {
	queryJSON, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	req := esapi.SearchRequest{
		Index: []string{indexName},
		Body:  bytes.NewReader(queryJSON),
	}

	res, err := req.Do(ctx, c.es)
	if err != nil {
		return nil, fmt.Errorf("failed to perform search: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("search error: %s", res.String())
	}

	var searchResponse SearchResponse
	if err := json.NewDecoder(res.Body).Decode(&searchResponse); err != nil {
		return nil, fmt.Errorf("failed to decode search response: %w", err)
	}

	return &searchResponse, nil
}

// Close closes the client
func (c *Client) Close() error {
	// Elasticsearch Go client doesn't require explicit closing
	return nil
}

// BulkOperation represents a bulk operation
type BulkOperation struct {
	Action     string      `json:"action"`     // index, create, update, delete
	Index      string      `json:"index"`
	DocumentID string      `json:"document_id"`
	Document   interface{} `json:"document,omitempty"`
}

// BulkResponse represents the response from a bulk operation
type BulkResponse struct {
	Took   int                      `json:"took"`
	Errors bool                     `json:"errors"`
	Items  []map[string]BulkResult  `json:"items"`
}

type BulkResult struct {
	Index   string                 `json:"_index"`
	ID      string                 `json:"_id"`
	Version int                    `json:"_version"`
	Result  string                 `json:"result"`
	Status  int                    `json:"status"`
	Error   map[string]interface{} `json:"error,omitempty"`
}

// SearchResponse represents the response from a search operation
type SearchResponse struct {
	Took     int  `json:"took"`
	TimedOut bool `json:"timed_out"`
	Hits     struct {
		Total struct {
			Value    int    `json:"value"`
			Relation string `json:"relation"`
		} `json:"total"`
		MaxScore float64     `json:"max_score"`
		Hits     []SearchHit `json:"hits"`
	} `json:"hits"`
	Aggregations map[string]interface{} `json:"aggregations,omitempty"`
	Suggest      map[string]interface{} `json:"suggest,omitempty"`
}

type SearchHit struct {
	Index     string                 `json:"_index"`
	ID        string                 `json:"_id"`
	Score     float64                `json:"_score"`
	Source    map[string]interface{} `json:"_source"`
	Highlight map[string][]string    `json:"highlight,omitempty"`
}