import { Link } from 'react-router-dom';

export default function NotFoundView() {
  return (
    <div
      id="not-found-view-wrapper"
      data-testid="not-found-view-wrapper"
      className="flex flex-col items-center justify-center h-full min-h-[400px] text-center space-y-4 select-none"
    >
      <h1 className="font-sans font-bold text-5xl text-[#3b82f6] tracking-tight">404</h1>
      <h2 className="font-sans font-semibold text-lg text-[#e4e7eb]">
        Ruta no encontrada
      </h2>
      <p className="font-sans text-sm text-[#9ba3b4] max-w-md">
        La vista solicitada no existe en este despliegue. Verifica la URL o regresa al
        panel principal.
      </p>
      <Link
        to="/dashboard"
        className="font-sans text-sm text-[#3b82f6] hover:text-[#60a5fa] underline underline-offset-4"
      >
        Volver al Dashboard
      </Link>
    </div>
  );
}
