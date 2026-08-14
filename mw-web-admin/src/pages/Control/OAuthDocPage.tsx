import { useQuery } from '@tanstack/react-query'
import { ArrowLeft, BookOpen } from 'lucide-react'
import { Link, useParams } from 'react-router-dom'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { PageHeader } from '@/components/control/page-header'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { controlRequest } from '@/lib/control-api'
import { usePreferences } from '@/preferences/preferences-context'

type DocumentationResponse = {
  version: string
  status: string
  content_type: string
  documentation: string
}

export default function OAuthDocPage({
  module = 'OAuth',
  basePath = '/oauth/docs',
}: {
  module?: 'OAuth' | 'Mail'
  basePath?: string
}) {
  const { version = 'v1' } = useParams()
  const { tr } = usePreferences()
  const query = useQuery({
    queryKey: [`${module.toLowerCase()}-docs`, version],
    queryFn: () =>
      controlRequest<DocumentationResponse>(
        `${basePath}/${encodeURIComponent(version)}`,
      ),
  })

  return (
    <main className="bg-background min-h-dvh px-4 py-7 sm:px-7 lg:px-10">
      <div className="mx-auto w-full max-w-5xl">
        <Button asChild className="mb-5" size="sm" variant="ghost">
          <Link to={basePath}>
            <ArrowLeft /> {tr('Documentação')}
          </Link>
        </Button>
        <PageHeader
          description={tr('Contrato público de integração do MW OAuth.')}
          eyebrow={module}
          icon={BookOpen}
          title={`${tr('Documentação da API')} · ${version}`}
        />
        <Card className="border-border/70 mt-7 overflow-hidden shadow-sm">
          {query.isLoading && (
            <p className="text-muted-foreground p-6 text-sm">
              {tr('Carregando dados…')}
            </p>
          )}
          {query.isError && (
            <p className="text-destructive p-6 text-sm">
              {tr('Não foi possível carregar a documentação.')}
            </p>
          )}
          {query.data && (
            <article className="text-card-foreground [&_a]:text-primary [&_blockquote]:border-brand-orange [&_code]:bg-muted [&_hr]:border-border [&_pre]:bg-muted [&_td]:border-border [&_th]:border-border p-6 text-sm leading-7 [&_a]:underline [&_a]:underline-offset-4 [&_blockquote]:my-4 [&_blockquote]:border-l-4 [&_blockquote]:pl-4 [&_code]:rounded [&_code]:px-1.5 [&_code]:py-0.5 [&_h1]:mb-5 [&_h1]:text-3xl [&_h1]:font-bold [&_h2]:mt-8 [&_h2]:mb-3 [&_h2]:text-2xl [&_h2]:font-semibold [&_h3]:mt-6 [&_h3]:mb-2 [&_h3]:text-lg [&_h3]:font-semibold [&_hr]:my-6 [&_li]:ml-5 [&_li]:list-disc [&_ol_li]:list-decimal [&_p]:my-3 [&_pre]:my-4 [&_pre]:overflow-x-auto [&_pre]:rounded-xl [&_pre]:p-4 [&_pre_code]:bg-transparent [&_pre_code]:p-0 [&_table]:my-5 [&_table]:w-full [&_table]:border-collapse [&_td]:border [&_th]:border">
              <ReactMarkdown remarkPlugins={[remarkGfm]}>
                {query.data.documentation}
              </ReactMarkdown>
            </article>
          )}
        </Card>
      </div>
    </main>
  )
}
