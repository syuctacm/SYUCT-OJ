import { Navigate, Route, Routes } from 'react-router-dom'
import { Layout } from './components/Layout.tsx'
import { ContestDetailPage } from './pages/ContestDetailPage.tsx'
import { ContestsPage } from './pages/ContestsPage.tsx'
import { LoginPage } from './pages/LoginPage.tsx'
import { ProblemDetailPage } from './pages/ProblemDetailPage.tsx'
import { ProblemsPage } from './pages/ProblemsPage.tsx'
import { RecordsPage } from './pages/RecordsPage.tsx'
import { SubmissionPage } from './pages/SubmissionPage.tsx'
import { TrainingDetailPage } from './pages/TrainingDetailPage.tsx'
import { TrainingsPage } from './pages/TrainingsPage.tsx'
import { SubmitPage } from './pages/SubmitPage.tsx'

export default function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route path="/" element={<Navigate to="/p" replace />} />
        <Route path="/p" element={<ProblemsPage />} />
        <Route path="/p/:pid" element={<ProblemDetailPage />} />
        <Route path="/p/:pid/submit" element={<SubmitPage />} />
        <Route path="/contest" element={<ContestsPage />} />
        <Route path="/contest/:tid" element={<ContestDetailPage />} />
        <Route path="/training" element={<TrainingsPage />} />
        <Route path="/training/:tid" element={<TrainingDetailPage />} />
        <Route path="/record" element={<RecordsPage />} />
        <Route path="/record/:rid" element={<SubmissionPage />} />
        <Route path="/login" element={<LoginPage />} />
        <Route path="*" element={<Navigate to="/p" replace />} />
      </Route>
    </Routes>
  )
}
