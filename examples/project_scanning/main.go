package main

import (
	"fmt"
	"log"

	engine "github.com/b-open-io/agent-master-engine"
)

func main() {
	// Create engine with file storage
	eng, err := engine.NewEngine(engine.WithFileStorage("~/.agent-master"))
	if err != nil {
		log.Fatalf("Failed to create engine: %v", err)
	}

	// Create project detector
	detector := engine.NewDefaultProjectDetector()

	// Scan for projects in common development directories
	scanPaths := []string{
		"~/code",
		"~/projects",
		"~/dev",
		".",
	}

	fmt.Println("Scanning for MCP projects...")
	projects, err := eng.ScanForProjects(scanPaths, detector)
	if err != nil {
		log.Fatalf("Failed to scan for projects: %v", err)
	}

	fmt.Printf("Found %d projects:\n\n", len(projects))

	for i, project := range projects {
		fmt.Printf("%d. %s\n", i+1, project.Name)
		fmt.Printf("   Path: %s\n", project.Path)
		fmt.Printf("   Servers: %d\n", len(project.Servers))

		if len(project.Servers) > 0 {
			fmt.Println("   MCP Servers:")
			for name, server := range project.Servers {
				fmt.Printf("     - %s (%s)\n", name, server.Transport)
				if server.Command != "" {
					fmt.Printf("       Command: %s\n", server.Command)
				}
				if server.URL != "" {
					fmt.Printf("       URL: %s\n", server.URL)
				}
			}
		}
		fmt.Println()
	}

	// Register the first project if found
	if len(projects) > 0 {
		project := projects[0]
		fmt.Printf("Registering project: %s\n", project.Name)

		err = eng.RegisterProject(project.Path, *project)
		if err != nil {
			log.Printf("Failed to register project: %v", err)
		} else {
			fmt.Println("Project registered successfully!")
		}

		// List all registered projects
		fmt.Println("\nRegistered projects:")
		projectInfos, err := eng.ListProjects()
		if err != nil {
			log.Printf("Failed to list projects: %v", err)
		} else {
			for _, info := range projectInfos {
				fmt.Printf("- %s (%d servers)\n", info.Name, info.ServerCount)
			}
		}
	}
}
