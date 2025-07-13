package config

// Deployment environments as constants to prevent typos
const (
	// EnvironmentProduction represents the production environment
	EnvironmentProduction = "production"
	
	// EnvironmentStaging represents the staging environment
	EnvironmentStaging = "staging"
	
	// EnvironmentDevelopment represents the development environment
	EnvironmentDevelopment = "development"
	
	// EnvironmentPRPreview represents PR preview environments
	EnvironmentPRPreview = "pr-preview"
	
	// EnvironmentTesting represents testing environments
	EnvironmentTesting = "testing"
)

// ValidEnvironments returns a list of all valid environment names
func ValidEnvironments() []string {
	return []string{
		EnvironmentProduction,
		EnvironmentStaging,
		EnvironmentDevelopment,
		EnvironmentPRPreview,
		EnvironmentTesting,
	}
}

// IsValidEnvironment checks if the given environment name is valid
func IsValidEnvironment(env string) bool {
	for _, validEnv := range ValidEnvironments() {
		if env == validEnv {
			return true
		}
	}
	return false
}