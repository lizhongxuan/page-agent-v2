#!/usr/bin/env node
import { execFileSync } from 'node:child_process'
import { cpSync, existsSync, mkdirSync, rmSync } from 'node:fs'
import { homedir } from 'node:os'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const rootDir = join(dirname(fileURLToPath(import.meta.url)), '..')
const desktopDir = join(homedir(), 'Desktop')
const extensionOutputDir = join(rootDir, 'packages/extension/.output')
const unpackedSourceDir = join(extensionOutputDir, 'chrome-mv3')
const zipSourcePath = join(extensionOutputDir, 'page-agent-ext-1.8.2-chrome.zip')
const unpackedTargetDir = join(desktopDir, 'page-agent-ext-1.8.2-chrome')
const zipTargetPath = join(desktopDir, 'page-agent-ext-1.8.2-chrome.zip')

console.log('Building Chrome extension...')
execFileSync('npm', ['run', 'build:ext'], { cwd: rootDir, stdio: 'inherit' })

if (!existsSync(unpackedSourceDir)) {
	throw new Error(`Missing unpacked extension output: ${unpackedSourceDir}`)
}
if (!existsSync(zipSourcePath)) {
	throw new Error(`Missing extension zip output: ${zipSourcePath}`)
}

mkdirSync(desktopDir, { recursive: true })
rmSync(unpackedTargetDir, { recursive: true, force: true })
rmSync(zipTargetPath, { force: true })

cpSync(unpackedSourceDir, unpackedTargetDir, { recursive: true })
cpSync(zipSourcePath, zipTargetPath)

console.log('\nExtension copied to desktop:')
console.log(`- Unpacked: ${unpackedTargetDir}`)
console.log(`- Zip: ${zipTargetPath}`)
