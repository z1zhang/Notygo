// Node loader hook: resolve extensionless TS imports (Vite-style) for smoke tests
export async function resolve(specifier, context, next) {
  try {
    return await next(specifier, context)
  } catch (e) {
    if (e.code === 'ERR_MODULE_NOT_FOUND' && !specifier.endsWith('.ts')) {
      return next(`${specifier}.ts`, context)
    }
    throw e
  }
}
