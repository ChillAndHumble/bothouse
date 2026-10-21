/** Decorative occupant for the channels sidebar entry. */
import { IconUsersOutlineRegular } from '@deepseek-ai/dsh-client-ui-primitives'
import type { PropsRuntime } from '@deepseek-ai/dsh-client-ui-slots'
import type {} from '@deepseek-ai/dsh-client-ui-sidebar/client'

/**
 * Render the members glyph at the size the sidebar asks for; the sidebar owns the label.
 * @param props - the sidebar's icon share.
 * @returns decorative icon.
 */
export function ChannelsIcon({ size }: PropsRuntime<'sidebar.panellist'>) {
  return <IconUsersOutlineRegular size={size} />
}
