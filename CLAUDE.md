# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Wails v2 application that combines a Go backend with a React TypeScript frontend. The application creates a desktop GUI with web technologies, where the frontend communicates with Go methods through Wails bindings.

## Development Commands

### Frontend Development
- `cd frontend && npm install` - Install frontend dependencies
- `cd frontend && npm run dev` - Start Vite development server for frontend-only development
- `cd frontend && npm run build` - Build frontend for production
- `cd frontend && tsc` - Run TypeScript compiler to check types

### Application Development
- `wails dev` - Run in live development mode with hot reload (recommended for full-stack development)
- `wails build` - Build production executable for current platform

### Development Server
When running `wails dev`, a development server is available at http://localhost:34115 for browser-based development with access to Go methods.

## Architecture

### Backend (Go)
- **main.go**: Application entry point, configures Wails app with window settings and binds the App struct
- **app.go**: Contains the main App struct with methods that are exposed to the frontend
- **Exposed Methods**: Go methods in the App struct are automatically available to the frontend through Wails bindings

### Frontend (React + TypeScript)
- **Technology Stack**: React 18 + TypeScript + Vite
- **Entry Point**: `frontend/src/main.tsx`
- **Main Component**: `frontend/src/App.tsx`
- **Go Bindings**: Generated TypeScript bindings in `frontend/wailsjs/` directory
  - `frontend/wailsjs/go/main/App.d.ts` - TypeScript definitions for Go App methods
  - `frontend/wailsjs/runtime/` - Wails runtime methods

### Build System
- **Wails Configuration**: `wails.json` defines build settings and frontend commands
- **Frontend Build**: Uses Vite with TypeScript compilation
- **Asset Embedding**: Frontend dist files are embedded into the Go binary using `//go:embed`
- **Build Artifacts**: Platform-specific build resources in `build/` directory

### Frontend-Backend Communication
- Go methods in the App struct are automatically exposed to the frontend
- Frontend imports generated bindings from `wailsjs/go/main/App`
- Example: `Greet(name).then(updateResultText)` calls the Go `Greet` method
- All communication is asynchronous and returns Promises

## Key Files
- `wails.json` - Wails project configuration
- `go.mod` - Go module dependencies
- `frontend/package.json` - Frontend dependencies and scripts
- `frontend/vite.config.ts` - Vite build configuration
- `main.go` - Application bootstrap and Wails configuration
- `app.go` - Backend logic and exposed methods