// @vitest-environment jsdom
import { describe, expect, it } from 'vitest'

import {
	ManualRecordingCapture,
	createRecordedActionTarget,
	normalizeManualDomEvent,
} from './manualRecording'

describe('manual recording capture', () => {
	it('creates stable target candidates in preferred order', () => {
		document.body.innerHTML = `
			<form>
				<label for="search">Search query</label>
				<input id="search" data-testid="search-box" placeholder="Search docs" />
				<button type="button">Save changes</button>
			</form>
		`

		const input = document.querySelector<HTMLInputElement>('#search')!
		const button = document.querySelector<HTMLButtonElement>('button')!
		const inputTarget = createRecordedActionTarget(input)

		expect(inputTarget).toMatchObject({
			testId: 'search-box',
			role: 'textbox',
			name: 'Search query',
		})
		expect(inputTarget.candidates?.slice(0, 4)).toMatchObject([
			{ strategy: 'testId', value: 'search-box' },
			{ strategy: 'role', role: 'textbox', name: 'Search query' },
			{ strategy: 'label', value: 'Search query' },
			{ strategy: 'placeholder', value: 'Search docs' },
		])

		expect(createRecordedActionTarget(button)).toMatchObject({
			role: 'button',
			name: 'Save changes',
			text: 'Save changes',
		})
	})

	it('adds css and xpath fallbacks when semantic candidates are unavailable', () => {
		document.body.innerHTML = `
			<section>
				<div>
					<span id="target-item"></span>
				</div>
			</section>
		`

		const target = createRecordedActionTarget(document.querySelector('#target-item')!)

		expect(target.css).toBe('#target-item')
		expect(target.xpath).toBe('/html[1]/body[1]/section[1]/div[1]/span[1]')
		expect(target.candidates?.map((candidate) => candidate.strategy)).toContain('css')
		expect(target.candidates?.map((candidate) => candidate.strategy)).toContain('xpath')
	})

	it('normalizes click events with page context and timestamp', () => {
		document.title = 'Recorder Test'
		document.body.innerHTML = '<button type="button" data-testid="save-button">Save</button>'

		const button = document.querySelector('button')!
		const event = new MouseEvent('click', { bubbles: true })
		button.dispatchEvent(event)

		const action = normalizeManualDomEvent(event, {
			now: () => 123,
			idFactory: () => 'a1',
		})

		expect(action).toMatchObject({
			id: 'a1',
			type: 'click',
			timestamp: 123,
			pageUrl: 'http://localhost:3000/',
			pageTitle: 'Recorder Test',
			target: {
				testId: 'save-button',
				role: 'button',
				name: 'Save',
			},
			result: 'success',
		})
	})

	it('coalesces input events per target and flushes only the final value', () => {
		document.body.innerHTML = '<label for="name">Name</label><input id="name" />'
		const input = document.querySelector<HTMLInputElement>('#name')!
		const emitted: unknown[] = []
		let id = 0
		const capture = new ManualRecordingCapture({
			onActions: (actions) => emitted.push(...actions),
			now: () => 100 + id,
			idFactory: () => `a${++id}`,
		})

		capture.start()
		input.value = 'J'
		input.dispatchEvent(new Event('input', { bubbles: true }))
		input.value = 'Jo'
		input.dispatchEvent(new Event('input', { bubbles: true }))
		input.value = 'Jon'
		input.dispatchEvent(new Event('input', { bubbles: true }))

		expect(emitted).toEqual([])

		const actions = capture.stop()
		expect(actions).toHaveLength(1)
		expect(actions[0]).toMatchObject({
			id: 'a3',
			type: 'input',
			value: 'Jon',
			target: { name: 'Name' },
		})
		expect(emitted).toHaveLength(1)
	})

	it('redacts sensitive values before actions leave the helper', () => {
		document.body.innerHTML =
			'<label for="password">Password</label><input id="password" type="password" />'
		const input = document.querySelector<HTMLInputElement>('#password')!
		const capture = new ManualRecordingCapture({
			idFactory: () => 'secret-action',
			now: () => 1,
		})

		capture.start()
		input.value = 'secret123'
		input.dispatchEvent(new Event('input', { bubbles: true }))
		const [action] = capture.stop()

		expect(action.value).toBe('[REDACTED]')
		expect(JSON.stringify(action)).not.toContain('secret123')
	})
})
