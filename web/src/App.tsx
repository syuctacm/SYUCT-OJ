import { Navigate, Route, Routes } from 'react-router-dom'
import { Layout } from './components/Layout.tsx'
import { AdminPage } from './pages/AdminPage.tsx'
import { AdminUsersPage } from './pages/AdminUsersPage.tsx'
import { ContestDetailPage } from './pages/ContestDetailPage.tsx'
import { ContestsPage } from './pages/ContestsPage.tsx'
import { DiscussionCreatePage } from './pages/DiscussionCreatePage.tsx'
import { DiscussionDetailPage } from './pages/DiscussionDetailPage.tsx'
import { DiscussionsPage } from './pages/DiscussionsPage.tsx'
import { HomeworkDetailPage } from './pages/HomeworkDetailPage.tsx'
import { HomePage } from './pages/HomePage.tsx'
import { HomeworksPage } from './pages/HomeworksPage.tsx'
import { LoginPage } from './pages/LoginPage.tsx'
import { ProblemDetailPage } from './pages/ProblemDetailPage.tsx'
import { ProblemsPage } from './pages/ProblemsPage.tsx'
import { RecordsPage } from './pages/RecordsPage.tsx'
import { SubmissionPage } from './pages/SubmissionPage.tsx'
import { UserProfilePage } from './pages/UserProfilePage.tsx'
import { TrainingDetailPage } from './pages/TrainingDetailPage.tsx'
import { TrainingsPage } from './pages/TrainingsPage.tsx'
import { SubmitPage } from './pages/SubmitPage.tsx'

export default function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route path="/" element={<HomePage />} />
        <Route path="/admin" element={<AdminPage />} />
        <Route path="/admin/users" element={<AdminUsersPage />} />
        <Route path="/p" element={<ProblemsPage />} />
        <Route path="/p/:pid" element={<ProblemDetailPage />} />
        <Route path="/p/:pid/submit" element={<SubmitPage />} />
        <Route path="/contest" element={<ContestsPage />} />
        <Route path="/contest/:tid" element={<ContestDetailPage />} />
        <Route path="/discuss" element={<DiscussionsPage />} />
        <Route path="/discuss/new" element={<DiscussionCreatePage />} />
        <Route path="/discuss/:did" element={<DiscussionDetailPage />} />
        <Route path="/homework" element={<HomeworksPage />} />
        <Route path="/homework/:tid" element={<HomeworkDetailPage />} />
        <Route path="/training" element={<TrainingsPage />} />
        <Route path="/training/:tid" element={<TrainingDetailPage />} />
        <Route path="/record" element={<RecordsPage />} />
        <Route path="/record/:rid" element={<SubmissionPage />} />
        <Route path="/user/:uid" element={<UserProfilePage />} />
        <Route path="/login" element={<LoginPage />} />
        <Route path="*" element={<Navigate to="/p" replace />} />
      </Route>
    </Routes>
  )
}
