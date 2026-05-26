export const PAGE_LOCK_ID = 'page-agent-v2-runtime-page-lock'

export function showPageLock() {
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
