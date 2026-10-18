import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { toast } from 'sonner'
import DOMPurify from 'dompurify'
import { marked } from 'marked'
import { FileText, Loader2, Pencil, Plus, Trash2 } from 'lucide-react'
import { useI18n } from '@/i18n'
import {
  createTemplate,
  deleteTemplate,
  listTemplates,
  previewTemplate,
  updateTemplate,
} from '@/lib/api'
import { getErrorMessage } from '@/lib/api-client'
import type { TemplateResponse } from '@/lib/types'
import { usePagination } from '@/hooks/use-pagination'
import { PageHeader } from '@/components/layout/PageHeader'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'
import { Textarea } from '@/components/ui/textarea'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Pagination } from '@/components/data-display/Pagination'
import { EmptyState } from '@/components/feedback/EmptyState'
import { ErrorState } from '@/components/feedback/ErrorState'
import { TableSkeleton } from '@/components/data-display/TableSkeleton'
import { ConfirmDialog } from '@/components/feedback/ConfirmDialog'
import { RefreshButton } from '@/components/feedback/RefreshButton'

const FORMATS = ['text', 'markdown', 'html'] as const

const DEFAULT_SAMPLE_DATA = '{\n  "event": {\n    "type": "alert",\n    "message": "Test notification"\n  }\n}'

const templateSchema = z.object({
  name: z.string().min(1),
  format: z.enum(FORMATS),
  title_template: z.string(),
  body_template: z.string(),
})

type TemplateForm = z.infer<typeof templateSchema>

const defaultForm: TemplateForm = {
  name: '',
  format: 'text',
  title_template: '',
  body_template: '',
}

interface RenderResult {
  title: string
  body: string
  format: string
}

/** Tailwind classes for rendered rich content (markdown/html) — no global styles needed. */
const RICH_CONTENT_CLASS =
  'space-y-1 break-words rounded-md bg-muted/50 p-3 text-sm leading-relaxed ' +
  '[&_a]:text-primary [&_a]:underline [&_blockquote]:border-l-4 [&_blockquote]:border-border [&_blockquote]:pl-3 [&_blockquote]:text-muted-foreground ' +
  '[&_code]:rounded [&_code]:bg-muted [&_code]:px-1 [&_h1]:mt-2 [&_h1]:text-base [&_h1]:font-semibold [&_h2]:mt-2 [&_h2]:text-base [&_h2]:font-semibold ' +
  '[&_h3]:font-semibold [&_h4]:font-semibold [&_img]:max-w-full [&_ol]:list-decimal [&_ol]:pl-5 ' +
  '[&_pre]:overflow-x-auto [&_pre]:rounded-md [&_pre]:bg-muted [&_pre]:p-2 [&_table]:w-full [&_td]:border [&_th]:border [&_ul]:list-disc [&_ul]:pl-5'

/** Sanitized rich-text body renderer (html/markdown) or plain pre-wrap text. */
function RenderedBody({ body, format }: { body: string; format: string }) {
  if (format === 'html') {
    return (
      <div
        className={RICH_CONTENT_CLASS}
        dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(body) }}
      />
    )
  }
  if (format === 'markdown') {
    const html = DOMPurify.sanitize(marked.parse(body, { async: false }))
    return <div className={RICH_CONTENT_CLASS} dangerouslySetInnerHTML={{ __html: html }} />
  }
  return <pre className="whitespace-pre-wrap break-words rounded-md bg-muted px-3 py-2 text-sm font-sans">{body}</pre>
}

export default function TemplatesPage() {
  const { t } = useI18n()
  const queryClient = useQueryClient()
  const { page, size, setPage, setSize } = usePagination()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<TemplateResponse | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<TemplateResponse | null>(null)

  // Preview state (kept visible while the dialog stays open)
  const [sampleData, setSampleData] = useState(DEFAULT_SAMPLE_DATA)
  const [previewResult, setPreviewResult] = useState<RenderResult | null>(null)
  const [rendering, setRendering] = useState(false)

  const listQuery = useQuery({
    queryKey: ['templates', page, size],
    queryFn: () => listTemplates({ page, size }),
  })

  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: ['templates'] })
    void queryClient.invalidateQueries({ queryKey: ['lookup', 'templates'] })
  }

  const form = useForm<TemplateForm>({
    resolver: zodResolver(templateSchema),
    defaultValues: defaultForm,
  })
  const format = form.watch('format')

  const saveMutation = useMutation({
    mutationFn: (values: TemplateForm) => {
      const payload: Record<string, unknown> = {
        name: values.name,
        format: values.format,
        title_template: values.title_template,
        body_template: values.body_template,
      }
      if (editing) {
        return updateTemplate(editing.id, payload)
      }
      return createTemplate(payload)
    },
    onSuccess: () => {
      toast.success(editing ? t('templates.templateUpdated') : t('templates.templateCreated'))
      setDialogOpen(false)
      invalidate()
    },
    onError: (e) => {
      toast.error(getErrorMessage(e, t('templates.failedToSaveTemplate')))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteTemplate(id),
    onSuccess: () => {
      toast.success(t('templates.templateDeleted'))
      setDeleteTarget(null)
      invalidate()
    },
    onError: (e) => {
      toast.error(getErrorMessage(e, t('common.failedToSave', { entity: t('nav.templates') })))
    },
  })

  function openCreate() {
    setEditing(null)
    form.reset(defaultForm)
    setSampleData(DEFAULT_SAMPLE_DATA)
    setPreviewResult(null)
    setDialogOpen(true)
  }

  function openEdit(row: TemplateResponse) {
    setEditing(row)
    form.reset({
      name: row.name ?? '',
      format: (FORMATS.includes(row.format as (typeof FORMATS)[number])
        ? row.format
        : 'text') as TemplateForm['format'],
      title_template: row.title_template ?? '',
      body_template: row.body_template ?? '',
    })
    setSampleData(DEFAULT_SAMPLE_DATA)
    setPreviewResult(null)
    setDialogOpen(true)
  }

  /** Render the current (possibly unsaved) form content against the sample data — pure preview, no DB writes. */
  async function handleRender() {
    const valid = await form.trigger()
    if (!valid) return

    const values = form.getValues()

    // Inline preview requires at least one template — check locally instead of
    // sending an empty spec the backend would reject (CONFIG_001).
    if (!values.title_template.trim() && !values.body_template.trim()) {
      toast.error(t('templates.contentRequired'))
      return
    }

    let data: Record<string, unknown>
    try {
      const parsed: unknown = JSON.parse(sampleData)
      if (parsed === null || typeof parsed !== 'object') {
        throw new Error('not an object')
      }
      data = parsed as Record<string, unknown>
    } catch {
      toast.error(t('templates.invalidJson'))
      return
    }

    setRendering(true)
    try {
      const res = await previewTemplate(
        {
          title_template: values.title_template,
          body_template: values.body_template,
          format: values.format,
        },
        data,
      )
      setPreviewResult({
        title: res.title ?? '',
        body: res.body ?? '',
        format: res.format ?? values.format,
      })
    } catch (e) {
      toast.error(getErrorMessage(e, t('templates.previewFailed')))
    } finally {
      setRendering(false)
    }
  }

  const rows = listQuery.data?.data ?? []
  const total = listQuery.data?.total ?? 0

  return (
    <div className="space-y-4">
      <PageHeader
        title={t('nav.templates')}
        description={t('templates.description')}
        actions={
          <>
            <RefreshButton
              label={t('dashboard.refresh')}
              onClick={() => void listQuery.refetch()}
              loading={listQuery.isFetching}
            />
            <Button size="sm" onClick={openCreate}>
              <Plus />
              {t('templates.newTemplate')}
            </Button>
          </>
        }
      />

      <div className="rounded-lg border">
        {listQuery.isPending ? (
          <TableSkeleton cols={4} />
        ) : listQuery.isError ? (
          <ErrorState
            error={listQuery.error}
            onRetry={() => void listQuery.refetch()}
            entity={t('nav.templates')}
          />
        ) : rows.length === 0 ? (
          <EmptyState
            title={t('templates.emptyTitle')}
            description={t('templates.emptyDescription')}
            icon={<FileText className="size-5" />}
            action={
              <Button size="sm" variant="outline" onClick={openCreate}>
                <Plus />
                {t('templates.newTemplate')}
              </Button>
            }
          />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('common.name')}</TableHead>
                <TableHead className="w-28">{t('templates.format')}</TableHead>
                <TableHead className="w-24">{t('mappings.revision')}</TableHead>
                <TableHead className="w-24 text-right">{t('common.actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((row) => (
                <TableRow key={row.id}>
                  <TableCell className="text-muted-foreground">{row.name}</TableCell>
                  <TableCell>
                    <Badge variant="secondary">
                      {t(`enums.templateFormat.${row.format}`, row.format)}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-muted-foreground">{row.revision}</TableCell>
                  <TableCell className="text-right">
                    <div className="flex justify-end gap-1">
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        onClick={() => openEdit(row)}
                        aria-label={t('common.edit')}
                      >
                        <Pencil />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        className="text-destructive hover:text-destructive"
                        onClick={() => setDeleteTarget(row)}
                        aria-label={t('common.delete')}
                      >
                        <Trash2 />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </div>

      {!listQuery.isPending && !listQuery.isError && total > 0 ? (
        <Pagination
          total={total}
          page={page}
          size={size}
          onPageChange={setPage}
          onSizeChange={setSize}
        />
      ) : null}

      {/* Create / edit dialog */}
      <Dialog
        open={dialogOpen}
        onOpenChange={(open) => {
          if (!saveMutation.isPending && !rendering) setDialogOpen(open)
        }}
      >
        <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-xl">
          <DialogHeader>
            <DialogTitle>
              {editing ? t('templates.editTemplate') : t('templates.newTemplate')}
            </DialogTitle>
            <DialogDescription className="sr-only">
              {editing ? t('templates.editTemplate') : t('templates.newTemplate')}
            </DialogDescription>
          </DialogHeader>

          <form
            className="space-y-4"
            onSubmit={form.handleSubmit((values) => saveMutation.mutate(values))}
          >
            <div className="grid grid-cols-[1fr_180px] gap-4">
              <div className="space-y-2">
                <Label htmlFor="template-name">{t('common.name')}</Label>
                <Input
                  id="template-name"
                  placeholder={t('templates.newTemplate')}
                  {...form.register('name')}
                />
                {form.formState.errors.name ? (
                  <p className="text-xs text-destructive">{t('templates.nameRequired')}</p>
                ) : null}
              </div>

              <div className="space-y-2">
                <Label>{t('templates.format')}</Label>
                <Select
                  value={format}
                  onValueChange={(v) => form.setValue('format', v as TemplateForm['format'])}
                >
                  <SelectTrigger className="w-full">
                    <SelectValue placeholder={t('templates.selectFormat')} />
                  </SelectTrigger>
                  <SelectContent>
                    {FORMATS.map((f) => (
                      <SelectItem key={f} value={f}>
                        {t('enums.templateFormat.' + f)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                {form.formState.errors.format ? (
                  <p className="text-xs text-destructive">{t('templates.formatRequired')}</p>
                ) : null}
              </div>
            </div>

            <div className="space-y-2">
              <Label htmlFor="template-title">{t('templates.titleTemplate')}</Label>
              <Textarea
                id="template-title"
                rows={3}
                placeholder={t('templates.titlePlaceholder')}
                {...form.register('title_template')}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="template-body">{t('templates.bodyTemplate')}</Label>
              <Textarea
                id="template-body"
                rows={10}
                spellCheck={false}
                placeholder={t('templates.bodyPlaceholder')}
                {...form.register('body_template')}
              />
            </div>

            <Separator />

            {/* Preview section */}
            <div className="space-y-3">
              <p className="text-sm font-medium">{t('templates.templatePreview')}</p>
              <div className="space-y-2">
                <Label htmlFor="template-sample">{t('templates.sampleData')}</Label>
                <Textarea
                  id="template-sample"
                  rows={4}
                  spellCheck={false}
                  placeholder='{"event": {"type": "alert", "message": "test"}}'
                  value={sampleData}
                  onChange={(e) => setSampleData(e.target.value)}
                />
              </div>

              {previewResult ? (
                <div className="space-y-3">
                  <div className="space-y-1.5">
                    <p className="text-xs font-medium text-muted-foreground">
                      {t('templates.title')}
                    </p>
                    <div className="whitespace-pre-wrap break-words rounded-md bg-muted px-3 py-2 text-sm">
                      {previewResult.title}
                    </div>
                  </div>
                  <div className="space-y-1.5">
                    <p className="text-xs font-medium text-muted-foreground">
                      {t('templates.body')}
                    </p>
                    <RenderedBody body={previewResult.body} format={previewResult.format} />
                  </div>
                </div>
              ) : null}
            </div>

            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                onClick={() => setDialogOpen(false)}
                disabled={saveMutation.isPending || rendering}
              >
                {t('common.cancel')}
              </Button>
              <Button
                type="button"
                variant="outline"
                disabled={saveMutation.isPending || rendering}
                onClick={() => void handleRender()}
              >
                {rendering ? <Loader2 className="animate-spin" /> : null}
                {t('templates.render')}
              </Button>
              <Button type="submit" disabled={saveMutation.isPending || rendering}>
                {saveMutation.isPending ? <Loader2 className="animate-spin" /> : null}
                {t('common.save')}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => {
          if (!deleteMutation.isPending) setDeleteTarget(open ? deleteTarget : null)
        }}
        title={t('common.confirmDelete')}
        description={t('common.confirmDeleteMsg', {
          entity: t('nav.templates'),
          name: deleteTarget?.name ?? '',
        })}
        confirmLabel={t('common.delete')}
        onConfirm={() => deleteTarget && deleteMutation.mutate(deleteTarget.id)}
        pending={deleteMutation.isPending}
      />
    </div>
  )
}
