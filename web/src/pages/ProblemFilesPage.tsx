import { useEffect, useRef, useState, type ChangeEvent } from 'react'
import { useParams } from 'react-router-dom'
import {
  deleteProblemAssets,
  getProblemAssets,
  uploadProblemAssets,
  type ProblemAsset,
  type ProblemAssetList,
} from '../api.ts'
import { ProblemSidebar } from '../components/ProblemSidebar.tsx'

type AssetKind = 'testdata' | 'additional'

function formatSize(size: number) {
  if (size >= 1024 * 1024) return `${(size / (1024 * 1024)).toFixed(1)} MiB`
  if (size >= 1024) return `${(size / 1024).toFixed(1)} KiB`
  return `${size} B`
}

function AssetTable({
  kind,
  assets,
  selected,
  onToggle,
  onToggleAll,
}: {
  kind: AssetKind
  assets: ProblemAsset[]
  selected: Set<string>
  onToggle: (name: string) => void
  onToggleAll: () => void
}) {
  return (
    <div className="section__body no-padding files" data-type={kind === 'testdata' ? 'testdata' : 'additional_file'}>
      {assets.length ? (
        <table className="data-table">
          <colgroup>
            <col className="col--checkbox" />
            <col className="col--name" />
            <col className="col--size" />
            <col className="col--operation" />
          </colgroup>
          <thead>
            <tr>
              <th className="col--checkbox">
                <label className="compact checkbox">
                  <input
                    type="checkbox"
                    checked={assets.length > 0 && assets.every((asset) => selected.has(asset.name))}
                    onChange={onToggleAll}
                  />
                </label>
              </th>
              <th className="col--name">文件名</th>
              <th className="col--size">大小</th>
              <th className="col--operation"><span className="icon icon-wrench" /></th>
            </tr>
          </thead>
          <tbody>
            {assets.map((asset) => (
              <tr data-filename={asset.name} data-size={asset.size} key={asset.name}>
                <td className="col--checkbox">
                  <label className="compact checkbox">
                    <input
                      type="checkbox"
                      checked={selected.has(asset.name)}
                      onChange={() => onToggle(asset.name)}
                    />
                  </label>
                </td>
                <td className="col--name"><a href={asset.downloadUrl}>{asset.name}</a></td>
                <td className="col--size">{formatSize(asset.size)}</td>
                <td className="col--operation">
                  <a href={asset.downloadUrl} className="icon icon-download" aria-label={`下载 ${asset.name}`} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      ) : (
        <div className="nothing-placeholder compact"><div className="nothing-icon" />当前没有文件</div>
      )}
    </div>
  )
}

export function ProblemFilesPage() {
  const { pid = '' } = useParams()
  const [data, setData] = useState<ProblemAssetList | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState('')
  const [selected, setSelected] = useState<Record<AssetKind, Set<string>>>({
    testdata: new Set(),
    additional: new Set(),
  })
  const testdataInput = useRef<HTMLInputElement>(null)
  const additionalInput = useRef<HTMLInputElement>(null)

  useEffect(() => {
    setError('')
    getProblemAssets(pid).then(setData).catch((err: unknown) => setError(err instanceof Error ? err.message : '题目文件加载失败'))
  }, [pid])

  function toggle(kind: AssetKind, name: string) {
    setSelected((current) => {
      const next = new Set(current[kind])
      if (next.has(name)) next.delete(name)
      else next.add(name)
      return { ...current, [kind]: next }
    })
  }

  function toggleAll(kind: AssetKind) {
    if (!data) return
    const assets = kind === 'testdata' ? data.testdata : data.additional
    setSelected((current) => {
      const allSelected = assets.length > 0 && assets.every((asset) => current[kind].has(asset.name))
      return { ...current, [kind]: allSelected ? new Set<string>() : new Set(assets.map((asset) => asset.name)) }
    })
  }

  async function upload(kind: AssetKind, event: ChangeEvent<HTMLInputElement>) {
    const files = Array.from(event.target.files ?? [])
    event.target.value = ''
    if (!files.length) return
    setBusy(`upload-${kind}`)
    setError('')
    try {
      const next = await uploadProblemAssets(pid, kind, files)
      setData(next)
      setSelected((current) => ({ ...current, [kind]: new Set<string>() }))
    } catch (err) {
      setError(err instanceof Error ? err.message : '文件上传失败')
    } finally {
      setBusy('')
    }
  }

  async function removeSelected(kind: AssetKind) {
    const names = Array.from(selected[kind])
    if (!names.length) return
    setBusy(`delete-${kind}`)
    setError('')
    try {
      const next = await deleteProblemAssets(pid, kind, names)
      setData(next)
      setSelected((current) => ({ ...current, [kind]: new Set<string>() }))
    } catch (err) {
      setError(err instanceof Error ? err.message : '文件删除失败')
    } finally {
      setBusy('')
    }
  }

  if (error && !data) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder">{error}</div></div></div>
  }
  if (!data) {
    return <div className="row"><div className="columns"><div className="nothing-placeholder"><div className="nothing-icon" />加载中...</div></div></div>
  }

  return (
    <div className="row">
      <div className="medium-5 columns" data-type="testdata">
        <div className="section">
          <div className="section__header">
            <h1 className="section__title">测试数据</h1>
            <div className="section__tools">
              <button className="primary rounded button" type="button" onClick={() => testdataInput.current?.click()} disabled={Boolean(busy)}>
                上传
              </button>
              <input ref={testdataInput} type="file" multiple hidden onChange={(event) => upload('testdata', event)} />
            </div>
          </div>
          <AssetTable
            kind="testdata"
            assets={data.testdata}
            selected={selected.testdata}
            onToggle={(name) => toggle('testdata', name)}
            onToggleAll={() => toggleAll('testdata')}
          />
          <div className="section__body">
            <button className="rounded button" type="button" onClick={() => removeSelected('testdata')} disabled={!selected.testdata.size || Boolean(busy)}>
              删除选中
            </button>
          </div>
        </div>
      </div>

      <div className="medium-4 columns" data-type="additional_file">
        <div className="section">
          <div className="section__header">
            <h1 className="section__title">附加文件</h1>
            <div className="section__tools">
              <button className="primary rounded button" type="button" onClick={() => additionalInput.current?.click()} disabled={Boolean(busy)}>
                上传
              </button>
              <input ref={additionalInput} type="file" multiple hidden onChange={(event) => upload('additional', event)} />
            </div>
          </div>
          <AssetTable
            kind="additional"
            assets={data.additional}
            selected={selected.additional}
            onToggle={(name) => toggle('additional', name)}
            onToggleAll={() => toggleAll('additional')}
          />
          <div className="section__body">
            <button className="rounded button" type="button" onClick={() => removeSelected('additional')} disabled={!selected.additional.size || Boolean(busy)}>
              删除选中
            </button>
          </div>
        </div>
      </div>

      <div className="medium-3 columns">
        <ProblemSidebar problem={data.problem} pageName="problem_files" />
      </div>
      {error ? <div className="columns"><blockquote className="warn"><p>{error}</p></blockquote></div> : null}
    </div>
  )
}
