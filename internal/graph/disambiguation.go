package graph

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
)

// Disambiguation handles entity normalization and merging
type Disambiguation struct {
	client *Client
	logger *slog.Logger
	// Common alias mappings (abbreviation -> canonical name)
	aliases map[string]string
}

// NewDisambiguation creates a new Disambiguation handler
func NewDisambiguation(client *Client, logger *slog.Logger) *Disambiguation {
	if logger == nil {
		logger = slog.Default()
	}
	return &Disambiguation{
		client:  client,
		logger:  logger.With("component", "graph-disambiguation"),
		aliases: defaultAliases(),
	}
}

// defaultAliases returns common technology abbreviation mappings
func defaultAliases() map[string]string {
	return map[string]string{
		// Infrastructure & Cloud
		"tf":     "terraform",
		"k8s":    "kubernetes",
		"aws":    "amazon web services",
		"gcp":    "google cloud platform",
		"azure":  "microsoft azure",
		"ec2":    "amazon ec2",
		"s3":     "amazon s3",
		"rds":    "amazon rds",
		"lambda": "aws lambda",
		"eks":    "elastic kubernetes service",
		"ecs":    "elastic container service",
		"gke":    "google kubernetes engine",
		"aks":    "azure kubernetes service",
		"vm":     "virtual machine",
		"vms":    "virtual machines",
		"iac":    "infrastructure as code",
		"ci/cd":  "continuous integration",
		"cicd":   "continuous integration",
		"gh":     "github",
		"gl":     "gitlab",

		// Programming Languages & Frameworks
		"js":    "javascript",
		"ts":    "typescript",
		"py":    "python",
		"rb":    "ruby",
		"go":    "golang",
		"rs":    "rust",
		"node":  "nodejs",
		"react": "reactjs",
		"vue":   "vuejs",
		"ng":    "angular",
		"dj":    "django",
		"ror":   "ruby on rails",
		"rails": "ruby on rails",

		// Databases
		"pg":       "postgresql",
		"postgres": "postgresql",
		"mysql":    "mysql",
		"mongo":    "mongodb",
		"redis":    "redis",
		"es":       "elasticsearch",
		"elastic":  "elasticsearch",
		"dynamodb": "amazon dynamodb",
		"falkordb": "falkordb",

		// Tools & Protocols
		"docker":  "docker",
		"mcp":     "model context protocol",
		"api":     "api",
		"rest":    "restful api",
		"grpc":    "grpc",
		"graphql": "graphql",
		"sql":     "sql",
		"nosql":   "nosql",
		"llm":     "large language model",
		"ai":      "artificial intelligence",
		"ml":      "machine learning",

		// HashiCorp
		"hashi":     "hashicorp",
		"hashicorp": "hashicorp",
		"consul":    "hashicorp consul",
		"vault":     "hashicorp vault",
		"nomad":     "hashicorp nomad",
		"packer":    "hashicorp packer",
	}
}

// NormalizeEntityName returns the canonical normalized form of an entity name
func (d *Disambiguation) NormalizeEntityName(name string) string {
	// Step 1: Trim whitespace and convert to lowercase
	normalized := strings.TrimSpace(strings.ToLower(name))

	// Step 2: Remove common noise characters but preserve meaningful ones
	// Keep hyphens, underscores, and periods as they can be meaningful
	normalized = regexp.MustCompile(`[^\w\s\-_./]`).ReplaceAllString(normalized, "")

	// Step 3: Collapse multiple spaces into one
	normalized = regexp.MustCompile(`\s+`).ReplaceAllString(normalized, " ")

	// Step 4: Check for known aliases and replace with canonical form
	if canonical, ok := d.aliases[normalized]; ok {
		normalized = canonical
	}

	return normalized
}

// GetCanonicalName returns the canonical name for an entity, resolving aliases
func (d *Disambiguation) GetCanonicalName(name string) string {
	normalized := strings.TrimSpace(strings.ToLower(name))
	if canonical, ok := d.aliases[normalized]; ok {
		return canonical
	}
	return normalized
}

// AddAlias adds a custom alias mapping
func (d *Disambiguation) AddAlias(alias, canonical string) {
	d.aliases[strings.ToLower(alias)] = strings.ToLower(canonical)
}

// DuplicateEntity represents a potential duplicate entity pair
type DuplicateEntity struct {
	Entity1    string `json:"entity1"`
	Entity2    string `json:"entity2"`
	Normalized string `json:"normalized"`
	FileCount1 int64  `json:"file_count1"`
	FileCount2 int64  `json:"file_count2"`
}

// FindDuplicateEntities finds entities that normalize to the same canonical form
func (d *Disambiguation) FindDuplicateEntities(ctx context.Context) ([]DuplicateEntity, error) {
	// Get all entities with their file counts
	query := `
		MATCH (e:Entity)
		OPTIONAL MATCH (e)<-[:MENTIONS]-(f:File)
		RETURN e.name, e.normalized, count(f) as file_count
		ORDER BY e.normalized
	`
	result, err := d.client.Query(ctx, query, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get entities; %w", err)
	}

	// Group by normalized form
	type entityInfo struct {
		name      string
		fileCount int64
	}
	groups := make(map[string][]entityInfo)

	for result.Next() {
		record := result.Record()
		name := record.GetString(0, "")
		normalized := record.GetString(1, "")
		fileCount := record.GetInt64(2, 0)

		// Apply our normalization (which includes alias resolution)
		canonical := d.NormalizeEntityName(name)

		groups[canonical] = append(groups[canonical], entityInfo{
			name:      name,
			fileCount: fileCount,
		})

		// Also group by existing normalized value for matching
		if canonical != normalized {
			groups[normalized] = append(groups[normalized], entityInfo{
				name:      name,
				fileCount: fileCount,
			})
		}
	}

	// Find groups with more than one entity
	var duplicates []DuplicateEntity
	seen := make(map[string]bool)

	for normalized, entities := range groups {
		if len(entities) > 1 {
			// Create pairs of duplicates
			for i := 0; i < len(entities)-1; i++ {
				for j := i + 1; j < len(entities); j++ {
					key := entities[i].name + "|" + entities[j].name
					if entities[j].name < entities[i].name {
						key = entities[j].name + "|" + entities[i].name
					}
					if seen[key] {
						continue
					}
					seen[key] = true

					duplicates = append(duplicates, DuplicateEntity{
						Entity1:    entities[i].name,
						Entity2:    entities[j].name,
						Normalized: normalized,
						FileCount1: entities[i].fileCount,
						FileCount2: entities[j].fileCount,
					})
				}
			}
		}
	}

	d.logger.Debug("found duplicate entities", "count", len(duplicates))
	return duplicates, nil
}

// MergeEntities merges one entity into another, redirecting all relationships
// The source entity is deleted after merging
func (d *Disambiguation) MergeEntities(ctx context.Context, sourceEntity, targetEntity string) (int64, error) {
	d.logger.Info("merging entities",
		"source", sourceEntity,
		"target", targetEntity,
	)

	// Step 1: Ensure target entity exists (normalize the name)
	normalizedTarget := d.NormalizeEntityName(targetEntity)

	// Step 2: Redirect all MENTIONS from source to target
	query := `
		MATCH (source:Entity {normalized: $sourceNormalized})<-[r:MENTIONS]-(f:File)
		MATCH (target:Entity {normalized: $targetNormalized})
		WHERE source <> target
		MERGE (f)-[:MENTIONS]->(target)
		DELETE r
		RETURN count(r) as redirected
	`
	params := map[string]any{
		"sourceNormalized": strings.ToLower(sourceEntity),
		"targetNormalized": normalizedTarget,
	}

	result, err := d.client.Query(ctx, query, params)
	if err != nil {
		return 0, fmt.Errorf("failed to redirect relationships; %w", err)
	}

	var redirected int64
	if result.Next() {
		redirected = result.Record().GetInt64(0, 0)
	}

	// Step 3: Delete the source entity if it has no remaining relationships
	deleteQuery := `
		MATCH (e:Entity {normalized: $normalized})
		WHERE NOT EXISTS((e)<-[:MENTIONS]-())
		DELETE e
		RETURN count(e) as deleted
	`
	_, err = d.client.Query(ctx, deleteQuery, map[string]any{
		"normalized": strings.ToLower(sourceEntity),
	})
	if err != nil {
		d.logger.Warn("failed to delete source entity",
			"entity", sourceEntity,
			"error", err,
		)
	}

	d.logger.Info("merged entities",
		"source", sourceEntity,
		"target", targetEntity,
		"redirected", redirected,
	)

	return redirected, nil
}

// NormalizeAllEntities applies normalization to all entities in the graph
// This updates the normalized field and merges entities that become duplicates
func (d *Disambiguation) NormalizeAllEntities(ctx context.Context) (int64, error) {
	d.logger.Info("normalizing all entities")

	// Get all entities
	query := `
		MATCH (e:Entity)
		RETURN e.name, e.normalized
	`
	result, err := d.client.Query(ctx, query, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to get entities; %w", err)
	}

	var updated int64
	for result.Next() {
		record := result.Record()
		name := record.GetString(0, "")
		currentNormalized := record.GetString(1, "")

		// Calculate new normalized form
		newNormalized := d.NormalizeEntityName(name)

		// Update if different
		if newNormalized != currentNormalized {
			updateQuery := `
				MATCH (e:Entity {normalized: $currentNormalized})
				WHERE e.name = $name
				SET e.normalized = $newNormalized
			`
			_, err := d.client.Query(ctx, updateQuery, map[string]any{
				"name":              name,
				"currentNormalized": currentNormalized,
				"newNormalized":     newNormalized,
			})
			if err != nil {
				d.logger.Warn("failed to update entity normalization",
					"entity", name,
					"error", err,
				)
				continue
			}
			updated++
		}
	}

	// After normalization, find and merge duplicates
	duplicates, err := d.FindDuplicateEntities(ctx)
	if err != nil {
		return updated, fmt.Errorf("failed to find duplicates after normalization; %w", err)
	}

	for _, dup := range duplicates {
		// Merge into the entity with more files (more established)
		var source, target string
		if dup.FileCount1 >= dup.FileCount2 {
			source = dup.Entity2
			target = dup.Entity1
		} else {
			source = dup.Entity1
			target = dup.Entity2
		}

		merged, err := d.MergeEntities(ctx, source, target)
		if err != nil {
			d.logger.Warn("failed to merge duplicate entities",
				"source", source,
				"target", target,
				"error", err,
			)
			continue
		}
		updated += merged
	}

	d.logger.Info("entity normalization complete",
		"updated", updated,
		"duplicates_merged", len(duplicates),
	)

	return updated, nil
}

// EntityVariant represents a variant of an entity name
type EntityVariant struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Normalized string `json:"normalized"`
	FileCount  int64  `json:"file_count"`
}

// GetEntityVariants returns all variant names for an entity
func (d *Disambiguation) GetEntityVariants(ctx context.Context, entityName string) ([]EntityVariant, error) {
	normalized := d.NormalizeEntityName(entityName)

	query := `
		MATCH (e:Entity)
		WHERE e.normalized = $normalized
		   OR toLower(e.name) = $entityLower
		OPTIONAL MATCH (e)<-[:MENTIONS]-(f:File)
		RETURN DISTINCT e.name, e.type, e.normalized, count(f) as file_count
		ORDER BY file_count DESC
	`
	params := map[string]any{
		"normalized":  normalized,
		"entityLower": strings.ToLower(entityName),
	}

	result, err := d.client.Query(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get entity variants; %w", err)
	}

	var variants []EntityVariant
	for result.Next() {
		record := result.Record()
		variants = append(variants, EntityVariant{
			Name:       record.GetString(0, ""),
			Type:       record.GetString(1, ""),
			Normalized: record.GetString(2, ""),
			FileCount:  record.GetInt64(3, 0),
		})
	}

	return variants, nil
}

// SimilarEntity represents an entity that might be similar to a query
type SimilarEntity struct {
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Normalized  string  `json:"normalized"`
	FileCount   int64   `json:"file_count"`
	Similarity  float64 `json:"similarity"`
	MatchReason string  `json:"match_reason"`
}

// FindSimilarEntities finds entities similar to the given name
// Uses prefix matching, substring matching, and alias resolution
func (d *Disambiguation) FindSimilarEntities(ctx context.Context, name string, limit int) ([]SimilarEntity, error) {
	normalized := d.NormalizeEntityName(name)
	nameLower := strings.ToLower(name)

	query := `
		MATCH (e:Entity)
		WHERE e.normalized STARTS WITH $prefix
		   OR e.normalized CONTAINS $nameLower
		   OR toLower(e.name) CONTAINS $nameLower
		OPTIONAL MATCH (e)<-[:MENTIONS]-(f:File)
		RETURN DISTINCT e.name, e.type, e.normalized, count(f) as file_count
		ORDER BY file_count DESC
		LIMIT $limit
	`
	params := map[string]any{
		"prefix":    normalized[:min(3, len(normalized))],
		"nameLower": nameLower,
		"limit":     limit,
	}

	result, err := d.client.Query(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to find similar entities; %w", err)
	}

	var similar []SimilarEntity
	for result.Next() {
		record := result.Record()
		entityName := record.GetString(0, "")
		entityNormalized := record.GetString(2, "")

		// Calculate match reason and similarity
		matchReason := "substring"
		similarity := 0.5

		if entityNormalized == normalized {
			matchReason = "exact_normalized"
			similarity = 1.0
		} else if strings.HasPrefix(entityNormalized, normalized) {
			matchReason = "prefix"
			similarity = 0.8
		} else if strings.Contains(entityNormalized, normalized) {
			matchReason = "contains"
			similarity = 0.6
		}

		similar = append(similar, SimilarEntity{
			Name:        entityName,
			Type:        record.GetString(1, ""),
			Normalized:  entityNormalized,
			FileCount:   record.GetInt64(3, 0),
			Similarity:  similarity,
			MatchReason: matchReason,
		})
	}

	return similar, nil
}
