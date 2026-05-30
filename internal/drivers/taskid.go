package drivers

import "strings"

// BuildTaskID builds a stable cross-platform task identifier.
func BuildTaskID(platform, providerTaskID string) string {
	platform = strings.TrimSpace(platform)
	providerTaskID = strings.TrimSpace(providerTaskID)
	if platform == "" || providerTaskID == "" {
		return providerTaskID
	}
	return platform + ":" + providerTaskID
}

// ParseTaskID splits a cross-platform task identifier into platform and provider task ID.
func ParseTaskID(taskID string) (platform string, providerTaskID string, ok bool) {
	taskID = strings.TrimSpace(taskID)
	platform, providerTaskID, found := strings.Cut(taskID, ":")
	if !found || strings.TrimSpace(platform) == "" || strings.TrimSpace(providerTaskID) == "" {
		return "", taskID, false
	}

	return strings.TrimSpace(platform), strings.TrimSpace(providerTaskID), true
}

// ProviderTaskID returns the provider-specific task ID, accepting either unified or legacy IDs.
func ProviderTaskID(platform, taskID string) string {
	taskPlatform, providerTaskID, ok := ParseTaskID(taskID)
	if !ok {
		return strings.TrimSpace(taskID)
	}
	if strings.EqualFold(taskPlatform, strings.TrimSpace(platform)) {
		return providerTaskID
	}
	return strings.TrimSpace(taskID)
}
