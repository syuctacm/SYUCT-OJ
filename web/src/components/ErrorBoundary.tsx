import { Component, type ErrorInfo, type ReactNode } from 'react'
import { ErrorPage } from '../pages/ErrorPage.tsx'

type Props = { children: ReactNode }
type State = { error: Error | null }

export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null }

  static getDerivedStateFromError(error: Error): State {
    return { error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('page render failed', error, info)
  }

  render() {
    if (this.state.error) {
      return <ErrorPage status={500} message="页面加载失败，请稍后重试。" />
    }
    return this.props.children
  }
}
