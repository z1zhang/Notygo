import { useState } from 'react'

export function usePagination(initialSize = 20) {
  const [page, setPage] = useState(1)
  const [size, setSize] = useState(initialSize)
  return { page, size, setPage, setSize }
}
