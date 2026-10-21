import { Navigate, Route, Routes } from 'react-router-dom'
import { Layout } from './components/Layout.tsx'
import { LegacyDomainRedirect } from './components/LegacyDomainRedirect.tsx'
import { AdminPage } from './pages/AdminPage.tsx'
import { ActivityFilesPage } from './pages/ActivityFilesPage.tsx'
import { AdminUsersPage } from './pages/AdminUsersPage.tsx'
import { ContestDetailPage } from './pages/ContestDetailPage.tsx'
import { ContestProblemsPage } from './pages/ContestProblemsPage.tsx'
import { ContestScoreboardPage } from './pages/ContestScoreboardPage.tsx'
import { ContestEditPage } from './pages/ContestEditPage.tsx'
import { ContestsPage } from './pages/ContestsPage.tsx'
import { DiscussionCreatePage } from './pages/DiscussionCreatePage.tsx'
import { DiscussionDetailPage } from './pages/DiscussionDetailPage.tsx'
import { DiscussionEditPage } from './pages/DiscussionEditPage.tsx'
import { DiscussionsPage } from './pages/DiscussionsPage.tsx'
import { HomeworkDetailPage } from './pages/HomeworkDetailPage.tsx'
import { HomeworkEditPage } from './pages/HomeworkEditPage.tsx'
import { HomePage } from './pages/HomePage.tsx'
import { HomeworksPage } from './pages/HomeworksPage.tsx'
import { LoginPage } from './pages/LoginPage.tsx'
import { ProblemConfigPage } from './pages/ProblemConfigPage.tsx'
import { ProblemDetailPage } from './pages/ProblemDetailPage.tsx'
import { ProblemEditPage } from './pages/ProblemEditPage.tsx'
import { ProblemFilesPage } from './pages/ProblemFilesPage.tsx'
import { ProblemsPage } from './pages/ProblemsPage.tsx'
import { RecordsPage } from './pages/RecordsPage.tsx'
import { SubmissionPage } from './pages/SubmissionPage.tsx'
import { UserProfilePage } from './pages/UserProfilePage.tsx'
import { TrainingDetailPage } from './pages/TrainingDetailPage.tsx'
import { TrainingEditPage } from './pages/TrainingEditPage.tsx'
import { TrainingsPage } from './pages/TrainingsPage.tsx'
import { SubmitPage } from './pages/SubmitPage.tsx'
import { TagRankingPage } from './pages/TagRankingPage.tsx'

export default function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route path="/" element={<HomePage />} />
        <Route path="/admin" element={<AdminPage />} />
        <Route path="/admin/users" element={<AdminUsersPage />} />
        <Route path="/p" element={<ProblemsPage />} />
        <Route path="/p/new" element={<ProblemEditPage mode="create" />} />
        <Route path="/p/tag/:tag" element={<TagRankingPage />} />
        <Route path="/p/:pid/edit" element={<ProblemEditPage mode="edit" />} />
        <Route path="/p/:pid/config" element={<ProblemConfigPage />} />
        <Route path="/p/:pid/files" element={<ProblemFilesPage />} />
        <Route path="/p/:pid" element={<ProblemDetailPage />} />
        <Route path="/p/:pid/submit" element={<SubmitPage />} />
        <Route path="/contest" element={<ContestsPage />} />
        <Route path="/contest/new" element={<ContestEditPage mode="create" />} />
        <Route path="/contest/:tid/edit" element={<ContestEditPage mode="edit" />} />
        <Route path="/contest/:tid/files" element={<ActivityFilesPage kind="contest" />} />
        <Route path="/contest/:tid/problems" element={<ContestProblemsPage />} />
        <Route path="/contest/:tid/scoreboard" element={<ContestScoreboardPage />} />
        <Route path="/contest/:tid" element={<ContestDetailPage />} />
        <Route path="/discuss" element={<DiscussionsPage />} />
        <Route path="/discuss/new" element={<DiscussionCreatePage />} />
        <Route path="/discuss/node/:name/create" element={<DiscussionCreatePage />} />
        <Route path="/discuss/:did/edit" element={<DiscussionEditPage />} />
        <Route path="/discuss/:did" element={<DiscussionDetailPage />} />
        <Route path="/homework" element={<HomeworksPage />} />
        <Route path="/homework/new" element={<HomeworkEditPage mode="create" />} />
        <Route path="/homework/:tid/edit" element={<HomeworkEditPage mode="edit" />} />
        <Route path="/homework/:tid/files" element={<ActivityFilesPage kind="homework" />} />
        <Route path="/homework/:tid/scoreboard" element={<ContestScoreboardPage />} />
        <Route path="/homework/:tid" element={<HomeworkDetailPage />} />
        <Route path="/training" element={<TrainingsPage />} />
        <Route path="/training/new" element={<TrainingEditPage mode="create" />} />
        <Route path="/training/:tid/edit" element={<TrainingEditPage mode="edit" />} />
        <Route path="/training/:tid/files" element={<ActivityFilesPage kind="training" />} />
        <Route path="/training/:tid" element={<TrainingDetailPage />} />
        <Route path="/record" element={<RecordsPage />} />
        <Route path="/record/:rid" element={<SubmissionPage />} />
        <Route path="/user/:uid" element={<UserProfilePage />} />
        <Route path="/login" element={<LoginPage />} />
        <Route path="/d/:domainId/*" element={<LegacyDomainRedirect />} />
        <Route path="*" element={<Navigate to="/p" replace />} />
      </Route>
    </Routes>
  )
}
