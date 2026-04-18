# Masters Rowing Category Calculator

A web application for calculating rowing masters categories based on crew members' ages. Built with Go and the Datastar reactive framework.

## Features

- Interactive web interface for adding/removing crew members
- Automatic masters category calculation based on age bands (A-K)
- Real-time updates using Server-Sent Events (SSE)
- Server-side session storage with NATS JetStream
- Responsive design with Datastar frontend
- Health check endpoint for monitoring

## Getting Started

### Using Dev Container (Recommended)

1. Open this repository in VS Code
2. When prompted, click "Reopen in Container" or use the Command Palette: `Dev Containers: Reopen in Container`
3. The container will build and install all dependencies automatically

### Local Development

Make sure you have Go 1.21+ installed:

```bash
# Install dependencies
go mod tidy

# Run the server
go run main.go

# Run tests
go test -v
```

## Endpoints

- `GET /masterscalc` - Main application interface for managing crew members
- `GET /masterscalc/rowers` - Server-sent events endpoint for real-time updates
- `POST /masterscalc/rowers` - Add a new rower to the crew
- `DELETE /masterscalc/rowers/{idx}` - Remove a rower from the crew by index
- `GET /health` - Health check endpoint
- `GET /static/*` - Static assets (CSS, etc.)

## Usage

1. Navigate to `http://localhost:8080/masterscalc` in your browser
2. Enter crew member details:
   - **Name**: Rower's name
   - **Birth Year or Age**: Either birth year (e.g., 1988) or current age (e.g., 37)
3. Click "Add" to add the rower to your crew
4. View calculated masters categories for each member
5. Remove crew members using the "Remove" button

## Masters Age Categories

The application calculates masters categories based on these age bands:

- **A**: 27-35 years
- **B**: 36-42 years  
- **C**: 43-49 years
- **D**: 50-54 years
- **E**: 55-59 years
- **F**: 60-64 years
- **G**: 65-69 years
- **H**: 70-74 years
- **I**: 75-79 years
- **J**: 80-84 years
- **K**: 85+ years

## Environment Variables

- `PORT` - Server port (default: 8080)
- `SESSION_SECRET` - Base64-encoded secret key for session management (required, generate with `go run cmd/sessionkey/main.go`)

## Technology Stack

- **Backend**: Go with standard library HTTP server
- **Frontend**: HTML templates with Datastar for reactivity
- **Data Storage**: Server-side NATS JetStream key-value store
- **Dependencies**:
  - `github.com/starfederation/datastar-go` - Server-sent events and reactive updates
  - `github.com/nats-io/nats.go` - NATS messaging system with JetStream
  - `github.com/delaneyj/toolbelt/embeddednats` - Embedded NATS server

## Development

The project includes:

- Go modules for dependency management
- Unit tests for handlers  
- VS Code dev container with Go tooling
- Automatic formatting and linting on save

## Architecture

- Server-side session storage using NATS JetStream key-value store
- Embedded NATS server for persistent data storage
- Server-sent events for real-time UI updates
- Template-based HTML rendering
- RESTful API design for crew management
