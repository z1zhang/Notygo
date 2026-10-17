import { marked } from "marked"
import DOMPurify from "dompurify"

interface RenderedBodyProps {
  body?: string | null
  format?: string | null
}

const boxClass = "rounded-md border bg-muted/40 p-3 text-sm break-words"

/**
 * Rendered message body: HTML and Markdown are sanitized with DOMPurify
 * before injection; anything else is shown as plain pre-wrapped text.
 */
export function RenderedBody({ body, format }: RenderedBodyProps) {
  if (!body) {
    return <span className="text-sm text-muted-foreground">-</span>
  }

  if (format === "html") {
    const html = DOMPurify.sanitize(body)
    return <div className={boxClass} dangerouslySetInnerHTML={{ __html: html }} />
  }

  if (format === "markdown") {
    const html = DOMPurify.sanitize(marked.parse(body) as string)
    return <div className={boxClass} dangerouslySetInnerHTML={{ __html: html }} />
  }

  return <pre className={`${boxClass} font-mono text-xs whitespace-pre-wrap`}>{body}</pre>
}
