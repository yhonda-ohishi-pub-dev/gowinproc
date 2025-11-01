#!/usr/bin/env node

/**
 * Protocol Buffers TypeScript Generation Script
 *
 * This script generates TypeScript types and service definitions from .proto files
 * using ts-proto. It handles the Windows-specific path issues with protoc plugins.
 */

const { execSync } = require('child_process')
const path = require('path')
const fs = require('fs')

// Configuration
const PROTO_DIR = path.join(__dirname, '../proto')
const OUTPUT_DIR = path.join(__dirname, '../src/proto')
const PROTO_FILE = 'process_manager.proto'

// Ensure directories exist
if (!fs.existsSync(OUTPUT_DIR)) {
  fs.mkdirSync(OUTPUT_DIR, { recursive: true })
}

// Copy proto file from backend to frontend
const BACKEND_PROTO = path.join(__dirname, '../../src/internal/proto', PROTO_FILE)
const FRONTEND_PROTO = path.join(PROTO_DIR, PROTO_FILE)

if (!fs.existsSync(PROTO_DIR)) {
  fs.mkdirSync(PROTO_DIR, { recursive: true })
}

console.log('Copying proto file from backend...')
fs.copyFileSync(BACKEND_PROTO, FRONTEND_PROTO)
console.log(`✓ Copied ${PROTO_FILE}`)

// Build protoc command
const pluginPath = path.join(__dirname, '../node_modules/.bin/protoc-gen-ts_proto.cmd')
const absolutePluginPath = path.resolve(pluginPath)

const command = `protoc ` +
  `--proto_path=${PROTO_DIR} ` +
  `--plugin=protoc-gen-ts_proto="${absolutePluginPath}" ` +
  `--ts_proto_out=${OUTPUT_DIR} ` +
  `--ts_proto_opt=outputServices=generic-definitions,outputServices=default,esModuleInterop=true ` +
  `${path.join(PROTO_DIR, PROTO_FILE)}`

console.log('\nGenerating TypeScript from proto...')
console.log(`Command: ${command}\n`)

try {
  execSync(command, { stdio: 'inherit' })
  console.log(`\n✓ Successfully generated TypeScript types in ${OUTPUT_DIR}`)
  console.log(`✓ Generated file: ${path.join(OUTPUT_DIR, 'process_manager.ts')}`)
} catch (error) {
  console.error('\n✗ Failed to generate TypeScript from proto')
  console.error(error.message)
  process.exit(1)
}
