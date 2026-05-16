import { useEffect } from 'react'

const SITE_NAME = '10xInterview'
const SITE_URL = 'https://10xinterview.com'
const DEFAULT_IMAGE = `${SITE_URL}/og-image.jpg`

export type SEOProps = {
  title: string
  description: string
  path: string
  image?: string
  noindex?: boolean
  type?: 'website' | 'article'
  jsonLd?: Record<string, unknown> | Record<string, unknown>[]
}

function setMeta(selector: string, attr: 'name' | 'property', key: string, value: string) {
  let el = document.head.querySelector<HTMLMetaElement>(selector)
  if (!el) {
    el = document.createElement('meta')
    el.setAttribute(attr, key)
    document.head.appendChild(el)
  }
  el.setAttribute('content', value)
}

function setName(name: string, value: string) {
  setMeta(`meta[name="${name}"]`, 'name', name, value)
}

function setProperty(prop: string, value: string) {
  setMeta(`meta[property="${prop}"]`, 'property', prop, value)
}

function setCanonical(href: string) {
  let el = document.head.querySelector<HTMLLinkElement>('link[rel="canonical"]')
  if (!el) {
    el = document.createElement('link')
    el.setAttribute('rel', 'canonical')
    document.head.appendChild(el)
  }
  el.setAttribute('href', href)
}

const JSONLD_ID = 'route-jsonld'

function setJsonLd(data: SEOProps['jsonLd']) {
  document.head.querySelector(`script[data-id="${JSONLD_ID}"]`)?.remove()
  if (!data) return
  const script = document.createElement('script')
  script.type = 'application/ld+json'
  script.dataset.id = JSONLD_ID
  script.text = JSON.stringify(data)
  document.head.appendChild(script)
}

export function useSEO({
  title,
  description,
  path,
  image = DEFAULT_IMAGE,
  noindex = false,
  type = 'website',
  jsonLd,
}: SEOProps) {
  useEffect(() => {
    const fullTitle = title.includes(SITE_NAME) ? title : `${title} | ${SITE_NAME}`
    const url = `${SITE_URL}${path.startsWith('/') ? path : `/${path}`}`

    document.title = fullTitle
    setName('description', description)
    setName('robots', noindex ? 'noindex, nofollow' : 'index, follow, max-image-preview:large, max-snippet:-1, max-video-preview:-1')

    setCanonical(url)

    setProperty('og:type', type)
    setProperty('og:site_name', SITE_NAME)
    setProperty('og:url', url)
    setProperty('og:title', fullTitle)
    setProperty('og:description', description)
    setProperty('og:image', image)

    setName('twitter:card', 'summary_large_image')
    setName('twitter:url', url)
    setName('twitter:title', fullTitle)
    setName('twitter:description', description)
    setName('twitter:image', image)

    setJsonLd(jsonLd)
  }, [title, description, path, image, noindex, type, jsonLd])
}
