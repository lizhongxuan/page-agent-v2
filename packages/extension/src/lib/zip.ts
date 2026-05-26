const UTF8_FLAG = 0x0800
const STORE_METHOD = 0
const VERSION_NEEDED = 20
const DOS_TIME = 0
const DOS_DATE = 0

export function createZipBlob(files: Record<string, string>): Blob {
	const encoder = new TextEncoder()
	const localParts: Uint8Array[] = []
	const centralParts: Uint8Array[] = []
	let offset = 0

	for (const [path, content] of Object.entries(files).sort(([a], [b]) => a.localeCompare(b))) {
		const name = encoder.encode(normalizeZipPath(path))
		const data = encoder.encode(content)
		const crc = crc32(data)
		const localHeader = createLocalFileHeader(name, data, crc)
		const centralHeader = createCentralDirectoryHeader(name, data, crc, offset)

		localParts.push(localHeader, data)
		centralParts.push(centralHeader)
		offset += localHeader.byteLength + data.byteLength
	}

	const centralDirectoryOffset = offset
	const centralDirectorySize = centralParts.reduce((total, part) => total + part.byteLength, 0)
	const endRecord = createEndOfCentralDirectoryRecord(
		centralParts.length,
		centralDirectorySize,
		centralDirectoryOffset
	)

	return new Blob([...localParts, ...centralParts, endRecord].map(toArrayBuffer), {
		type: 'application/zip',
	})
}

function toArrayBuffer(part: Uint8Array): ArrayBuffer {
	const copy = new Uint8Array(part.byteLength)
	copy.set(part)
	return copy.buffer
}

function createLocalFileHeader(name: Uint8Array, data: Uint8Array, crc: number) {
	const header = new Uint8Array(30 + name.byteLength)
	const view = new DataView(header.buffer)

	view.setUint32(0, 0x04034b50, true)
	view.setUint16(4, VERSION_NEEDED, true)
	view.setUint16(6, UTF8_FLAG, true)
	view.setUint16(8, STORE_METHOD, true)
	view.setUint16(10, DOS_TIME, true)
	view.setUint16(12, DOS_DATE, true)
	view.setUint32(14, crc, true)
	view.setUint32(18, data.byteLength, true)
	view.setUint32(22, data.byteLength, true)
	view.setUint16(26, name.byteLength, true)
	view.setUint16(28, 0, true)
	header.set(name, 30)

	return header
}

function createCentralDirectoryHeader(
	name: Uint8Array,
	data: Uint8Array,
	crc: number,
	localHeaderOffset: number
) {
	const header = new Uint8Array(46 + name.byteLength)
	const view = new DataView(header.buffer)

	view.setUint32(0, 0x02014b50, true)
	view.setUint16(4, VERSION_NEEDED, true)
	view.setUint16(6, VERSION_NEEDED, true)
	view.setUint16(8, UTF8_FLAG, true)
	view.setUint16(10, STORE_METHOD, true)
	view.setUint16(12, DOS_TIME, true)
	view.setUint16(14, DOS_DATE, true)
	view.setUint32(16, crc, true)
	view.setUint32(20, data.byteLength, true)
	view.setUint32(24, data.byteLength, true)
	view.setUint16(28, name.byteLength, true)
	view.setUint16(30, 0, true)
	view.setUint16(32, 0, true)
	view.setUint16(34, 0, true)
	view.setUint16(36, 0, true)
	view.setUint32(38, 0, true)
	view.setUint32(42, localHeaderOffset, true)
	header.set(name, 46)

	return header
}

function createEndOfCentralDirectoryRecord(
	fileCount: number,
	centralDirectorySize: number,
	centralDirectoryOffset: number
) {
	const header = new Uint8Array(22)
	const view = new DataView(header.buffer)

	view.setUint32(0, 0x06054b50, true)
	view.setUint16(4, 0, true)
	view.setUint16(6, 0, true)
	view.setUint16(8, fileCount, true)
	view.setUint16(10, fileCount, true)
	view.setUint32(12, centralDirectorySize, true)
	view.setUint32(16, centralDirectoryOffset, true)
	view.setUint16(20, 0, true)

	return header
}

function normalizeZipPath(path: string) {
	return path.replace(/\\/g, '/').replace(/^\/+/, '')
}

function crc32(data: Uint8Array) {
	let crc = 0xffffffff

	for (const byte of data) {
		crc ^= byte
		for (let bit = 0; bit < 8; bit++) {
			crc = crc & 1 ? (crc >>> 1) ^ 0xedb88320 : crc >>> 1
		}
	}

	return (crc ^ 0xffffffff) >>> 0
}
