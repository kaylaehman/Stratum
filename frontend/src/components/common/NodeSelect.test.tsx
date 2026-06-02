import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'

import { NodeSelect } from './NodeSelect'

const nodes = [
  { id: 'b', name: 'beta', host: '10.0.0.2' },
  { id: 'a', name: 'alpha', host: '10.0.0.1' },
]

describe('NodeSelect', () => {
  it('sorts options by name and can append the host', () => {
    render(<NodeSelect nodes={nodes} value="a" onChange={() => {}} showHost />)
    const options = screen.getAllByRole('option')
    expect(options.map((o) => o.textContent)).toEqual(['alpha (10.0.0.1)', 'beta (10.0.0.2)'])
  })

  it('omits the host when showHost is false', () => {
    render(<NodeSelect nodes={nodes} value="a" onChange={() => {}} />)
    expect(screen.getByRole('option', { name: 'alpha' })).toBeInTheDocument()
  })

  it('preserves caller order when sort=false', () => {
    render(<NodeSelect nodes={nodes} value="b" onChange={() => {}} sort={false} />)
    const options = screen.getAllByRole('option')
    expect(options.map((o) => o.textContent)).toEqual(['beta', 'alpha'])
  })

  it('shows the empty label when there are no nodes', () => {
    render(<NodeSelect nodes={[]} value="" onChange={() => {}} emptyLabel="No nodes" />)
    expect(screen.getByRole('option', { name: 'No nodes' })).toBeInTheDocument()
  })

  it('reports the selected node id on change', async () => {
    const onChange = vi.fn()
    render(<NodeSelect nodes={nodes} value="a" onChange={onChange} />)
    await userEvent.selectOptions(screen.getByRole('combobox'), 'b')
    expect(onChange).toHaveBeenCalledWith('b')
  })
})
