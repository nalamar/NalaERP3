-- Personal-Stammdaten, Teams, Urlaubsanträge, Abwesenheiten
CREATE TABLE IF NOT EXISTS hr_teams (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    lead_employee_id UUID
);

CREATE TABLE IF NOT EXISTS hr_employees (
    id UUID PRIMARY KEY,
    personalnummer TEXT UNIQUE,
    first_name TEXT NOT NULL,
    last_name TEXT NOT NULL,
    email TEXT,
    phone TEXT,
    hire_date DATE,
    termination_date DATE,
    role TEXT,
    team_id UUID REFERENCES hr_teams(id),
    location TEXT,
    cost_center TEXT,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_hr_employees_active ON hr_employees(active);

ALTER TABLE hr_teams
    ADD CONSTRAINT fk_hr_teams_lead FOREIGN KEY (lead_employee_id) REFERENCES hr_employees(id) DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE IF NOT EXISTS hr_leave_requests (
    id UUID PRIMARY KEY,
    employee_id UUID NOT NULL REFERENCES hr_employees(id) ON DELETE CASCADE,
    typ TEXT NOT NULL, -- vacation | sick | unpaid | other
    status TEXT NOT NULL DEFAULT 'pending', -- pending | approved | rejected
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    days NUMERIC(6,2) NOT NULL DEFAULT 0,
    reason TEXT,
    approver_id UUID REFERENCES hr_employees(id),
    decided_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_hr_leave_requests_employee ON hr_leave_requests(employee_id, status);

CREATE TABLE IF NOT EXISTS hr_absences (
    id UUID PRIMARY KEY,
    employee_id UUID NOT NULL REFERENCES hr_employees(id) ON DELETE CASCADE,
    typ TEXT NOT NULL,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    status TEXT NOT NULL DEFAULT 'approved',
    source_request_id UUID REFERENCES hr_leave_requests(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_hr_absences_employee ON hr_absences(employee_id, start_date);

CREATE TABLE IF NOT EXISTS hr_holidays (
    id UUID PRIMARY KEY,
    location TEXT NOT NULL DEFAULT 'DE',
    date DATE NOT NULL,
    name TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_hr_holidays_loc_date ON hr_holidays(location, date);
