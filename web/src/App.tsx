import { lazy } from 'react'
import { Route, Routes } from 'react-router-dom'
import { Layout } from './components/Layout.tsx'
import { LegacyDomainRedirect } from './components/LegacyDomainRedirect.tsx'
const AdminPage = lazy(() => import('./pages/AdminPage.tsx').then((module) => ({ default: module.AdminPage })))
const ActivityFilesPage = lazy(() => import('./pages/ActivityFilesPage.tsx').then((module) => ({ default: module.ActivityFilesPage })))
const ContestManagementPage = lazy(() => import('./pages/ContestManagementPage.tsx').then((module) => ({ default: module.ContestManagementPage })))
const AdminUsersPage = lazy(() => import('./pages/AdminUsersPage.tsx').then((module) => ({ default: module.AdminUsersPage })))
const AdminUserImportPage = lazy(() => import('./pages/AdminUserImportPage.tsx').then((module) => ({ default: module.AdminUserImportPage })))
const AdminSettingsPage = lazy(() => import('./pages/AdminSettingsPage.tsx').then((module) => ({ default: module.AdminSettingsPage })))
const AdminAuditPage = lazy(() => import('./pages/AdminAuditPage.tsx').then((module) => ({ default: module.AdminAuditPage })))
const ContestDetailPage = lazy(() => import('./pages/ContestDetailPage.tsx').then((module) => ({ default: module.ContestDetailPage })))
const ContestClarificationPage = lazy(() => import('./pages/ContestClarificationPage.tsx').then((module) => ({ default: module.ContestClarificationPage })))
const ContestBalloonPage = lazy(() => import('./pages/ContestBalloonPage.tsx').then((module) => ({ default: module.ContestBalloonPage })))
const ContestProblemsPage = lazy(() => import('./pages/ContestProblemsPage.tsx').then((module) => ({ default: module.ContestProblemsPage })))
const ContestPrintPage = lazy(() => import('./pages/ContestPrintPage.tsx').then((module) => ({ default: module.ContestPrintPage })))
const ContestScoreboardPage = lazy(() => import('./pages/ContestScoreboardPage.tsx').then((module) => ({ default: module.ContestScoreboardPage })))
const ContestUserPage = lazy(() => import('./pages/ContestUserPage.tsx').then((module) => ({ default: module.ContestUserPage })))
const RankingPage = lazy(() => import('./pages/RankingPage.tsx').then((module) => ({ default: module.RankingPage })))
const StatusPage = lazy(() => import('./pages/StatusPage.tsx').then((module) => ({ default: module.StatusPage })))
const AboutPage = lazy(() => import('./pages/AboutPage.tsx').then((module) => ({ default: module.AboutPage })))
const HelpPage = lazy(() => import('./pages/HelpPage.tsx').then((module) => ({ default: module.HelpPage })))
import { NotFoundPage } from './pages/ErrorPage.tsx'
const UserDeletePage = lazy(() => import('./pages/UserDeletePage.tsx').then((module) => ({ default: module.UserDeletePage })))
const ContestEditPage = lazy(() => import('./pages/ContestEditPage.tsx').then((module) => ({ default: module.ContestEditPage })))
const ContestsPage = lazy(() => import('./pages/ContestsPage.tsx').then((module) => ({ default: module.ContestsPage })))
const DiscussionCreatePage = lazy(() => import('./pages/DiscussionCreatePage.tsx').then((module) => ({ default: module.DiscussionCreatePage })))
const DiscussionDetailPage = lazy(() => import('./pages/DiscussionDetailPage.tsx').then((module) => ({ default: module.DiscussionDetailPage })))
const DiscussionEditPage = lazy(() => import('./pages/DiscussionEditPage.tsx').then((module) => ({ default: module.DiscussionEditPage })))
const DiscussionsPage = lazy(() => import('./pages/DiscussionsPage.tsx').then((module) => ({ default: module.DiscussionsPage })))
const HomeworkDetailPage = lazy(() => import('./pages/HomeworkDetailPage.tsx').then((module) => ({ default: module.HomeworkDetailPage })))
const HomeworkEditPage = lazy(() => import('./pages/HomeworkEditPage.tsx').then((module) => ({ default: module.HomeworkEditPage })))
import { HomePage } from './pages/HomePage.tsx'
const HomeSettingsPage = lazy(() => import('./pages/HomeSettingsPage.tsx').then((module) => ({ default: module.HomeSettingsPage })))
const EmailChangeConfirmPage = lazy(() => import('./pages/EmailChangeConfirmPage.tsx').then((module) => ({ default: module.EmailChangeConfirmPage })))
const HomeworksPage = lazy(() => import('./pages/HomeworksPage.tsx').then((module) => ({ default: module.HomeworksPage })))
import { LoginPage } from './pages/LoginPage.tsx'
const LostPassPage = lazy(() => import('./pages/LostPassPage.tsx').then((module) => ({ default: module.LostPassPage })))
const LostPassCodePage = lazy(() => import('./pages/LostPassCodePage.tsx').then((module) => ({ default: module.LostPassCodePage })))
const RegisterPage = lazy(() => import('./pages/RegisterPage.tsx').then((module) => ({ default: module.RegisterPage })))
const RegisterCodePage = lazy(() => import('./pages/RegisterCodePage.tsx').then((module) => ({ default: module.RegisterCodePage })))
const MessagesPage = lazy(() => import('./pages/MessagesPage.tsx').then((module) => ({ default: module.MessagesPage })))
const SecurityPage = lazy(() => import('./pages/SecurityPage.tsx').then((module) => ({ default: module.SecurityPage })))
const ProblemConfigPage = lazy(() => import('./pages/ProblemConfigPage.tsx').then((module) => ({ default: module.ProblemConfigPage })))
const ProblemDetailPage = lazy(() => import('./pages/ProblemDetailPage.tsx').then((module) => ({ default: module.ProblemDetailPage })))
const ProblemEditPage = lazy(() => import('./pages/ProblemEditPage.tsx').then((module) => ({ default: module.ProblemEditPage })))
const ProblemImportFpsPage = lazy(() => import('./pages/ProblemImportFpsPage.tsx').then((module) => ({ default: module.ProblemImportFpsPage })))
const ProblemImportHydroPage = lazy(() => import('./pages/ProblemImportHydroPage.tsx').then((module) => ({ default: module.ProblemImportHydroPage })))
const ProblemFilesPage = lazy(() => import('./pages/ProblemFilesPage.tsx').then((module) => ({ default: module.ProblemFilesPage })))
const RandomProblemPage = lazy(() => import('./pages/RandomProblemPage.tsx').then((module) => ({ default: module.RandomProblemPage })))
const ProblemSolutionPage = lazy(() => import('./pages/ProblemSolutionPage.tsx').then((module) => ({ default: module.ProblemSolutionPage })))
const ProblemStatisticsPage = lazy(() => import('./pages/ProblemStatisticsPage.tsx').then((module) => ({ default: module.ProblemStatisticsPage })))
const ProblemsPage = lazy(() => import('./pages/ProblemsPage.tsx').then((module) => ({ default: module.ProblemsPage })))
const RecordsPage = lazy(() => import('./pages/RecordsPage.tsx').then((module) => ({ default: module.RecordsPage })))
const SubmissionPage = lazy(() => import('./pages/SubmissionPage.tsx').then((module) => ({ default: module.SubmissionPage })))
const UserProfilePage = lazy(() => import('./pages/UserProfilePage.tsx').then((module) => ({ default: module.UserProfilePage })))
const UserFilesPage = lazy(() => import('./pages/UserFilesPage.tsx').then((module) => ({ default: module.UserFilesPage })))
const TrainingDetailPage = lazy(() => import('./pages/TrainingDetailPage.tsx').then((module) => ({ default: module.TrainingDetailPage })))
const TrainingEditPage = lazy(() => import('./pages/TrainingEditPage.tsx').then((module) => ({ default: module.TrainingEditPage })))
const TrainingsPage = lazy(() => import('./pages/TrainingsPage.tsx').then((module) => ({ default: module.TrainingsPage })))
const SubmitPage = lazy(() => import('./pages/SubmitPage.tsx').then((module) => ({ default: module.SubmitPage })))
const TagRankingPage = lazy(() => import('./pages/TagRankingPage.tsx').then((module) => ({ default: module.TagRankingPage })))

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
