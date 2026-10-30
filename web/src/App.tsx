import { Route, Routes } from 'react-router-dom'
import { Layout } from './components/Layout.tsx'
import { LegacyDomainRedirect } from './components/LegacyDomainRedirect.tsx'
import { AdminPage } from './pages/AdminPage.tsx'
import { ActivityFilesPage } from './pages/ActivityFilesPage.tsx'
import { ContestManagementPage } from './pages/ContestManagementPage.tsx'
import { AdminUsersPage } from './pages/AdminUsersPage.tsx'
import { AdminUserImportPage } from './pages/AdminUserImportPage.tsx'
import { AdminSettingsPage } from './pages/AdminSettingsPage.tsx'
import { AdminAuditPage } from './pages/AdminAuditPage.tsx'
import { ContestDetailPage } from './pages/ContestDetailPage.tsx'
import { ContestClarificationPage } from './pages/ContestClarificationPage.tsx'
import { ContestBalloonPage } from './pages/ContestBalloonPage.tsx'
import { ContestProblemsPage } from './pages/ContestProblemsPage.tsx'
import { ContestPrintPage } from './pages/ContestPrintPage.tsx'
import { ContestScoreboardPage } from './pages/ContestScoreboardPage.tsx'
import { ContestUserPage } from './pages/ContestUserPage.tsx'
import { RankingPage } from './pages/RankingPage.tsx'
import { StatusPage } from './pages/StatusPage.tsx'
import { AboutPage } from './pages/AboutPage.tsx'
import { HelpPage } from './pages/HelpPage.tsx'
import { NotFoundPage } from './pages/ErrorPage.tsx'
import { UserDeletePage } from './pages/UserDeletePage.tsx'
import { ContestEditPage } from './pages/ContestEditPage.tsx'
import { ContestsPage } from './pages/ContestsPage.tsx'
import { DiscussionCreatePage } from './pages/DiscussionCreatePage.tsx'
import { DiscussionDetailPage } from './pages/DiscussionDetailPage.tsx'
import { DiscussionEditPage } from './pages/DiscussionEditPage.tsx'
import { DiscussionsPage } from './pages/DiscussionsPage.tsx'
import { HomeworkDetailPage } from './pages/HomeworkDetailPage.tsx'
import { HomeworkEditPage } from './pages/HomeworkEditPage.tsx'
import { HomePage } from './pages/HomePage.tsx'
import { HomeSettingsPage } from './pages/HomeSettingsPage.tsx'
import { EmailChangeConfirmPage } from './pages/EmailChangeConfirmPage.tsx'
import { HomeworksPage } from './pages/HomeworksPage.tsx'
import { LoginPage } from './pages/LoginPage.tsx'
import { LostPassPage } from './pages/LostPassPage.tsx'
import { LostPassCodePage } from './pages/LostPassCodePage.tsx'
import { RegisterPage } from './pages/RegisterPage.tsx'
import { RegisterCodePage } from './pages/RegisterCodePage.tsx'
import { MessagesPage } from './pages/MessagesPage.tsx'
import { SecurityPage } from './pages/SecurityPage.tsx'
import { ProblemConfigPage } from './pages/ProblemConfigPage.tsx'
import { ProblemDetailPage } from './pages/ProblemDetailPage.tsx'
import { ProblemEditPage } from './pages/ProblemEditPage.tsx'
import { ProblemImportFpsPage } from './pages/ProblemImportFpsPage.tsx'
import { ProblemImportHydroPage } from './pages/ProblemImportHydroPage.tsx'
import { ProblemFilesPage } from './pages/ProblemFilesPage.tsx'
import { RandomProblemPage } from './pages/RandomProblemPage.tsx'
import { ProblemSolutionPage } from './pages/ProblemSolutionPage.tsx'
import { ProblemStatisticsPage } from './pages/ProblemStatisticsPage.tsx'
import { ProblemsPage } from './pages/ProblemsPage.tsx'
import { RecordsPage } from './pages/RecordsPage.tsx'
import { SubmissionPage } from './pages/SubmissionPage.tsx'
import { UserProfilePage } from './pages/UserProfilePage.tsx'
import { UserFilesPage } from './pages/UserFilesPage.tsx'
import { TrainingDetailPage } from './pages/TrainingDetailPage.tsx'
import { TrainingEditPage } from './pages/TrainingEditPage.tsx'
import { TrainingsPage } from './pages/TrainingsPage.tsx'
import { SubmitPage } from './pages/SubmitPage.tsx'
import { TagRankingPage } from './pages/TagRankingPage.tsx'

function DevRenderFailure(): never {
  throw new Error('forced render failure for the local error-boundary check')
}

export default function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route path="/" element={<HomePage />} />
        <Route path="/admin" element={<AdminPage />} />
        <Route path="/admin/users" element={<AdminUsersPage />} />
        <Route path="/admin/users/import" element={<AdminUserImportPage />} />
        <Route path="/admin/settings" element={<AdminSettingsPage />} />
        <Route path="/admin/audit" element={<AdminAuditPage />} />
        <Route path="/p" element={<ProblemsPage />} />
        <Route path="/p/new" element={<ProblemEditPage mode="create" />} />
        <Route path="/problem/random" element={<RandomProblemPage />} />
        <Route path="/problem/import/fps" element={<ProblemImportFpsPage />} />
        <Route path="/problem/import/hydro" element={<ProblemImportHydroPage />} />
        <Route path="/p/tag/:tag" element={<TagRankingPage />} />
        <Route path="/p/:pid/edit" element={<ProblemEditPage mode="edit" />} />
        <Route path="/p/:pid/config" element={<ProblemConfigPage />} />
        <Route path="/p/:pid/files" element={<ProblemFilesPage />} />
        <Route path="/p/:pid/solution" element={<ProblemSolutionPage />} />
        <Route path="/p/:pid/solution/:sid" element={<ProblemSolutionPage />} />
        <Route path="/p/:pid/statistics" element={<ProblemStatisticsPage />} />
        <Route path="/p/:pid" element={<ProblemDetailPage />} />
        <Route path="/p/:pid/submit" element={<SubmitPage />} />
        <Route path="/contest" element={<ContestsPage />} />
        <Route path="/contest/new" element={<ContestEditPage mode="create" />} />
        <Route path="/contest/:tid/edit" element={<ContestEditPage mode="edit" />} />
        <Route path="/contest/:tid/management" element={<ContestManagementPage />} />
        <Route path="/contest/:tid/files" element={<ContestManagementPage />} />
        <Route path="/contest/:tid/problems" element={<ContestProblemsPage />} />
        <Route path="/contest/:tid/clarification" element={<ContestClarificationPage />} />
        <Route path="/contest/:tid/user" element={<ContestUserPage />} />
        <Route path="/contest/:tid/balloon" element={<ContestBalloonPage />} />
        <Route path="/contest/:tid/print" element={<ContestPrintPage />} />
        <Route path="/contest/:tid/scoreboard" element={<ContestScoreboardPage />} />
        <Route path="/contest/:tid" element={<ContestDetailPage />} />
        <Route path="/discuss" element={<DiscussionsPage />} />
        <Route path="/discuss/new" element={<DiscussionCreatePage />} />
        <Route path="/discuss/node/:name/create" element={<DiscussionCreatePage />} />
        <Route path="/discuss/node/:name" element={<DiscussionsPage />} />
        <Route path="/discuss/:type/:name/create" element={<DiscussionCreatePage />} />
        <Route path="/discuss/:type/:name" element={<DiscussionsPage />} />
        <Route path="/discuss/:did/edit" element={<DiscussionEditPage />} />
        <Route path="/discuss/:did" element={<DiscussionDetailPage />} />
        <Route path="/homework" element={<HomeworksPage />} />
        <Route path="/homework/new" element={<HomeworkEditPage mode="create" />} />
        <Route path="/homework/:tid/edit" element={<HomeworkEditPage mode="edit" />} />
        <Route path="/homework/:tid/file" element={<ActivityFilesPage kind="homework" />} />
        <Route path="/homework/:tid/files" element={<ActivityFilesPage kind="homework" />} />
        <Route path="/homework/:tid/scoreboard" element={<ContestScoreboardPage />} />
        <Route path="/homework/:tid" element={<HomeworkDetailPage />} />
        <Route path="/training" element={<TrainingsPage />} />
        <Route path="/training/new" element={<TrainingEditPage mode="create" />} />
        <Route path="/training/:tid/edit" element={<TrainingEditPage mode="edit" />} />
        <Route path="/training/:tid/file" element={<ActivityFilesPage kind="training" />} />
        <Route path="/training/:tid/files" element={<ActivityFilesPage kind="training" />} />
        <Route path="/training/:tid" element={<TrainingDetailPage />} />
        <Route path="/record" element={<RecordsPage />} />
        <Route path="/record/:rid" element={<SubmissionPage />} />
        <Route path="/user/:uid" element={<UserProfilePage />} />
        <Route path="/ranking" element={<RankingPage />} />
        <Route path="/status" element={<StatusPage />} />
        <Route path="/about" element={<AboutPage />} />
        <Route path="/wiki/help" element={<HelpPage />} />
        <Route path="/user/delete" element={<UserDeletePage />} />
        <Route path="/home/messages" element={<MessagesPage />} />
        <Route path="/home/security" element={<SecurityPage />} />
        <Route path="/home/changeMail/:code" element={<EmailChangeConfirmPage />} />
        <Route path="/home/settings" element={<HomeSettingsPage />} />
        <Route path="/file" element={<UserFilesPage />} />
        <Route path="/login" element={<LoginPage />} />
        <Route path="/lostpass" element={<LostPassPage />} />
        <Route path="/lostpass/:code" element={<LostPassCodePage />} />
        <Route path="/register" element={<RegisterPage />} />
        <Route path="/register/:code" element={<RegisterCodePage />} />
        <Route path="/d/:domainId/*" element={<LegacyDomainRedirect />} />
        {import.meta.env.DEV ? <Route path="/__error-test" element={<DevRenderFailure />} /> : null}
        <Route path="*" element={<NotFoundPage />} />
      </Route>
    </Routes>
  )
}
