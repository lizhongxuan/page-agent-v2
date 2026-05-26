import type { InteractionResponse } from '@/webops/interactions/interactionTypes'

const NO_USER_ANSWER =
	'用户没有提供补充信息。请不要自动继续或重复询问；如果缺少明确目标，请调用 done 结束任务，并说明需要用户提供具体要做什么。'

export function formatAskUserResponse(response: InteractionResponse): string {
	if (response.type === 'input') return response.value || NO_USER_ANSWER
	if (response.type === 'choice') return `用户选择了：${response.optionId}`
	if (response.type === 'rejected') return '用户指出当前目标控件错误，请尝试下一个候选。'
	if (response.type === 'manual_select') {
		return `用户手动指示了目标位置：viewport(${Math.round(response.x)}, ${Math.round(response.y)})。请基于该位置重新判断目标控件。`
	}
	if (response.type === 'handover_done') return '用户已完成页面接管，可以继续。'
	if (response.type === 'cancelled') return `用户未提供答案：${response.reason}`

	return NO_USER_ANSWER
}
