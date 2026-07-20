package service

import (
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
)

// Confidence describes certainty only. A recipe remains declarative data for a
// later reviewed Project/Service configuration step; selection never executes it.
type Confidence string

const (
	ConfidenceHigh    Confidence = "high"
	ConfidenceMedium  Confidence = "medium"
	ConfidenceLow     Confidence = "low"
	ConfidenceUnknown Confidence = "unknown"
)

type Recipe struct {
	Install, Build, Start     []string
	Port                      int
	Image, BuildImage, Output string
	Manual                    bool
}
type StackSignals struct{ Files, Dependencies, Scripts []string }
type StackDefinition struct {
	ID, Category string
	Signals      StackSignals
	Recipe       Recipe
	Confidence   Confidence
	Evidence     []string
}

// StackSelectionInput is a read-only repository snapshot. It has no Project,
// Service, Seal, or database ID: suggestions must be explicitly applied to a
// project-owned Service by the canonical configuration workflow.
type StackSelectionInput struct{ Files map[string][]byte }

var unknownStack = StackDefinition{ID: "unknown", Category: "manual", Confidence: ConfidenceUnknown, Evidence: []string{"no unambiguous supported stack matched; provide reviewed manual configuration"}, Recipe: Recipe{Manual: true}}

func javascriptStack(id string, deps []string, port int, output string) StackDefinition {
	recipe := PackageManagerRecipe(PackageManagerNPM)
	recipe.Build = []string{"npm", "run", "build"}
	recipe.Start = []string{"npm", "run", "start"}
	recipe.Port, recipe.Image, recipe.BuildImage, recipe.Output = port, "node:20-alpine", "node:20-alpine", output
	return StackDefinition{ID: id, Category: "frontend", Signals: StackSignals{Files: []string{"package.json"}, Dependencies: deps, Scripts: []string{"build", "start"}}, Confidence: ConfidenceMedium, Evidence: []string{"package manifest, framework evidence, and build/start scripts are required"}, Recipe: recipe}
}

// stackRegistry is data only. User-authored container configuration is manual
// authority, not permission for automatic command execution.
var stackRegistry = []StackDefinition{
	javascriptStack("nextjs", []string{"next"}, 3000, ".next"),
	javascriptStack("react-vite", []string{"react", "vite"}, 4173, "dist"),
	javascriptStack("vue-vite", []string{"vue", "vite"}, 4173, "dist"),
	javascriptStack("node-web", nil, 3000, "dist"),
	{ID: "spring-boot-maven", Category: "backend", Signals: StackSignals{Files: []string{"pom.xml"}, Dependencies: []string{"spring-boot"}}, Confidence: ConfidenceMedium, Evidence: []string{"Maven manifest and Spring Boot dependency are required"}, Recipe: Recipe{Install: []string{"mvn", "dependency:go-offline"}, Start: []string{"java", "-jar", "app.jar"}, Port: 8080, Image: "eclipse-temurin:21-jre-alpine", BuildImage: "maven:3.9-eclipse-temurin-21", Output: "target"}},
	{ID: "spring-boot-gradle", Category: "backend", Signals: StackSignals{Files: []string{"build.gradle*", "settings.gradle*"}, Dependencies: []string{"spring-boot"}}, Confidence: ConfidenceMedium, Evidence: []string{"Gradle build/settings files and Spring Boot dependency are required"}, Recipe: Recipe{Install: []string{"gradle", "dependencies"}, Start: []string{"java", "-jar", "app.jar"}, Port: 8080, Image: "eclipse-temurin:21-jre-alpine", BuildImage: "gradle:8-jdk21", Output: "build/libs"}},
	{ID: "static-nginx", Category: "static", Signals: StackSignals{Files: []string{"nginx.conf", "dist"}}, Confidence: ConfidenceMedium, Evidence: []string{"static output and Nginx configuration are required"}, Recipe: Recipe{Port: 80, Image: "nginx:1.27-alpine", Output: "dist"}},
	{ID: "dockerfile", Category: "container", Signals: StackSignals{Files: []string{"Dockerfile"}}, Confidence: ConfidenceHigh, Evidence: []string{"Dockerfile is user-supplied build configuration"}, Recipe: Recipe{Manual: true}},
	{ID: "compose", Category: "container", Signals: StackSignals{Files: []string{"compose.yaml", "compose.yml", "docker-compose.yml", "docker-compose.yaml"}}, Confidence: ConfidenceHigh, Evidence: []string{"Compose file is user-supplied deployment configuration"}, Recipe: Recipe{Manual: true}},
	unknownStack,
}

func StackDefinitions() []StackDefinition {
	result := make([]StackDefinition, len(stackRegistry))
	for i, definition := range stackRegistry {
		result[i] = cloneStackDefinition(definition)
	}
	return result
}
func StackDefinitionByID(id string) StackDefinition {
	for _, definition := range stackRegistry {
		if definition.ID == id {
			return cloneStackDefinition(definition)
		}
	}
	return cloneStackDefinition(unknownStack)
}

// SelectStackDefinition selects only one unambiguous convention from snapshot
// evidence. Compose/Dockerfile are authoritative only when singular.
func SelectStackDefinition(input StackSelectionInput) StackDefinition {
	files := normalizeStackFiles(input.Files)
	compose, dockerfile := hasAnyFile(files, "compose.yaml", "compose.yml", "docker-compose.yml", "docker-compose.yaml"), hasFile(files, "Dockerfile")
	if compose && dockerfile {
		return unknownStackWithEvidence("Compose and Dockerfile are conflicting user-authored authorities")
	}
	if compose {
		return StackDefinitionByID("compose")
	}
	if dockerfile {
		return StackDefinitionByID("dockerfile")
	}
	dependencies, scripts := repositorySignals(files)
	type candidate struct {
		definition  StackDefinition
		specificity int
	}
	var candidates []candidate
	for _, definition := range stackRegistry {
		if definition.ID != "unknown" && definition.ID != "compose" && definition.ID != "dockerfile" && matchesStack(definition.Signals, files, dependencies, scripts) {
			candidates = append(candidates, candidate{definition, len(definition.Signals.Files) + len(definition.Signals.Dependencies) + len(definition.Signals.Scripts)})
		}
	}
	if len(candidates) == 0 {
		return unknownStackWithEvidence("no supported stack signals matched repository snapshot")
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].specificity == candidates[j].specificity {
			return candidates[i].definition.ID < candidates[j].definition.ID
		}
		return candidates[i].specificity > candidates[j].specificity
	})
	for _, contender := range candidates[1:] {
		if !strictlyMoreSpecific(candidates[0].definition.Signals, contender.definition.Signals) {
			return unknownStackWithEvidence("multiple incompatible supported stacks match: " + candidates[0].definition.ID + ", " + contender.definition.ID)
		}
	}
	return cloneStackDefinition(candidates[0].definition)
}

func normalizeStackFiles(input map[string][]byte) map[string][]byte {
	files := make(map[string][]byte, len(input))
	for path, content := range input {
		files[filepath.ToSlash(filepath.Clean(path))] = content
	}
	return files
}
func hasFile(files map[string][]byte, want string) bool {
	for path := range files {
		if filepath.Base(path) == want {
			return true
		}
	}
	return false
}
func hasAnyFile(files map[string][]byte, names ...string) bool {
	for _, name := range names {
		if hasFile(files, name) {
			return true
		}
	}
	return false
}
func matchesStack(signals StackSignals, files map[string][]byte, dependencies, scripts map[string]bool) bool {
	for _, pattern := range signals.Files {
		matched := false
		for path := range files {
			base := filepath.Base(path)
			if path == pattern || strings.HasPrefix(path, pattern+"/") {
				matched = true
				break
			}
			if ok, _ := filepath.Match(pattern, base); ok {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	for _, dependency := range signals.Dependencies {
		if !dependencies[dependency] {
			return false
		}
	}
	for _, script := range signals.Scripts {
		if !scripts[script] {
			return false
		}
	}
	return true
}
func strictlyMoreSpecific(top, other StackSignals) bool {
	return len(top.Files)+len(top.Dependencies)+len(top.Scripts) > len(other.Files)+len(other.Dependencies)+len(other.Scripts) && containsSignals(top.Files, other.Files) && containsSignals(top.Dependencies, other.Dependencies) && containsSignals(top.Scripts, other.Scripts)
}
func containsSignals(values, required []string) bool {
	for _, item := range required {
		found := false
		for _, value := range values {
			if value == item {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
func repositorySignals(files map[string][]byte) (map[string]bool, map[string]bool) {
	dependencies, scripts := map[string]bool{}, map[string]bool{}
	for path, content := range files {
		if filepath.Base(path) == "package.json" {
			var pkg struct {
				Dependencies    map[string]string `json:"dependencies"`
				DevDependencies map[string]string `json:"devDependencies"`
				Scripts         map[string]string `json:"scripts"`
			}
			if json.Unmarshal(content, &pkg) == nil {
				for name := range pkg.Dependencies {
					dependencies[name] = true
				}
				for name := range pkg.DevDependencies {
					dependencies[name] = true
				}
				for name := range pkg.Scripts {
					scripts[name] = true
				}
			}
			continue
		}
		text := strings.ToLower(string(content))
		if strings.Contains(text, "spring-boot") || strings.Contains(text, "springframework.boot") {
			dependencies["spring-boot"] = true
		}
	}
	return dependencies, scripts
}
func unknownStackWithEvidence(reason string) StackDefinition {
	result := cloneStackDefinition(unknownStack)
	result.Evidence = []string{reason, unknownStack.Evidence[0]}
	return result
}
func cloneStackDefinition(definition StackDefinition) StackDefinition {
	definition.Signals.Files = append([]string(nil), definition.Signals.Files...)
	definition.Signals.Dependencies = append([]string(nil), definition.Signals.Dependencies...)
	definition.Signals.Scripts = append([]string(nil), definition.Signals.Scripts...)
	definition.Recipe.Install = append([]string(nil), definition.Recipe.Install...)
	definition.Recipe.Build = append([]string(nil), definition.Recipe.Build...)
	definition.Recipe.Start = append([]string(nil), definition.Recipe.Start...)
	definition.Evidence = append([]string(nil), definition.Evidence...)
	return definition
}
