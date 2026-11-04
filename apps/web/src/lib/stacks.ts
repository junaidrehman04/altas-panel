export type StackDefinition = {
  kind: string
  title: string
  summary: string
  deployable: boolean
}

export type StackCatalog = {
  projects_root: string
  stacks: StackDefinition[]
}

export type ComposeProject = {
  name: string
  workdir?: string
  containers: number
  running: number
  status: string
}

export type ComposeReport = {
  enabled: boolean
  available: boolean
  projects: ComposeProject[]
  message?: string
}
