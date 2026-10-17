import { useMemo } from 'react'
import DOMPurify from 'dompurify'
import { marked } from 'marked'
import { cn } from '@/lib/utils'

interface RenderedBodyProps {
  body: string
  format: string
  className?: string
}

/**
 * Renders a delivery preview body: HTML/Markdown are sanitized with DOMPurify
 * before injecting; plain text renders in a <pre>.
 */
export function RenderedBody({ body, format, className }: RenderedBodyProps) {
  const html = useMemo(() => {
    if (format === 'html') return DOMPurify.sanitize(body)
    if (format === 'markdown') {
      const parsed = marked.parse(body, { async: false })
      return DOMPurify.sanitize(typeof parsed === 'string' ? parsed : '')
    }
    return null
  }, [body, format])

  if (html === null) {
    return (
      <pre
        className={cn(
          'overflow-auto rounded-md border bg-muted/40 p-3 text-sm whitespace-pre-wrap break-words',
          className,
        )}
      >
        {body}
      </pre>
    )
  }

  return (
    <div
      className={cn(
        'rendered-body rounded-md border bg-muted/20 p-3 text-sm leading-relaxed break-words',
        '[&_h1]:mb-2 [&_h1]:mt-3 [&_h1]:text-base [&_h1]:font-semibold',
        '[&_h2]:mb-2 [&_h2]:mt-3 [&_h2]:text-sm [&_h2]:font-semibold',
        '[&_h3]:mb-1 [&_h3]:mt-2 [&_h3]:font-semibold',
        '[&_p]:my-1.5',
        '[&_ul]:my-1.5 [&_ul]:list-disc [&_ul]:pl-5',
        '[&_ol]:my-1.5 [&_ol]:list-decimal [&_ol]:pl-5',
        '[&_li]:my-0.5',
        '[&_code]:rounded-sm [&_code]:bg-muted [&_code]:px-1 [&_code]:py-0.5 [&_code]:font-mono [&_code]:text-xs',
        '[&_pre]:my-2 [&_pre]:overflow-x-auto [&_pre]:rounded-md [&_pre]:bg-muted [&_pre]:p-2 [&_pre_code]:bg-transparent [&_pre_code]:p-0',
        '[&_blockquote]:my-2 [&_blockquote]:border-l-2 [&_blockquote]:border-border [&_blockquote]:pl-3 [&_blockquote]:text-muted-foreground',
        '[&_table]:my-2 [&_table]:w-full [&_table]:border-collapse [&_th]:border [&_th]:border-border [&_th]:px-2 [&_th]:py-1 [&_td]:border [&_td]:border-border [&_td]:px-2 [&_td]:py-1',
        '[&_a]:text-primary [&_a]:underline',
        '[&_img]:max-w-full',
        className,
      )}
      // Content is sanitized with DOMPurify above — safe to inject
      dangerouslySetInnerHTML={{ __html: html }}
    />
  )
}
