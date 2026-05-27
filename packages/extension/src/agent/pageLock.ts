export const PAGE_LOCK_ID = 'page-agent-v2-runtime-page-lock'

let suspendCount = 0

export function showPageLock() {
	if (isPageLockSuspended()) {
		hidePageLock()
		return
	}

	if (document.getElementById(PAGE_LOCK_ID)) return

	const lock = document.createElement('div')
	lock.id = PAGE_LOCK_ID
	lock.setAttribute('data-browser-use-ignore', 'true')
	lock.setAttribute('data-page-agent-ignore', 'true')
	Object.assign(lock.style, {
		position: 'fixed',
		inset: '0',
		zIndex: '2147483640',
		pointerEvents: 'auto',
		cursor: 'wait',
		background: 'rgba(15, 23, 42, 0.02)',
	})

	for (const eventName of ['click', 'mousedown', 'mouseup', 'mousemove', 'wheel', 'touchstart']) {
		lock.addEventListener(
			eventName,
			(event) => {
				event.preventDefault()
				event.stopPropagation()
			},
			{ passive: false }
		)
	}

	document.body.appendChild(lock)
}

export function hidePageLock() {
	document.getElementById(PAGE_LOCK_ID)?.remove()
}

export function suspendPageLock() {
	suspendCount += 1
	hidePageLock()
	window.dispatchEvent(new CustomEvent('PageAgent::EnablePassThrough'))
}

export function resumePageLock() {
	suspendCount = Math.max(0, suspendCount - 1)
	if (suspendCount === 0) {
		window.dispatchEvent(new CustomEvent('PageAgent::DisablePassThrough'))
	}
}

export function isPageLockSuspended() {
	return suspendCount > 0
}

export async function withPageLockBypassed<T>(run: () => Promise<T>): Promise<T> {
	const lock = document.getElementById(PAGE_LOCK_ID) as HTMLElement | null
	if (!lock) return run()

	const previousPointerEvents = lock.style.pointerEvents
	lock.style.pointerEvents = 'none'

	try {
		return await run()
	} finally {
		lock.style.pointerEvents = previousPointerEvents
	}
}
