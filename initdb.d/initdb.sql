create table pessoas (
    apelido varchar(32) primary key not null,
    uid uuid not null,
    nome  varchar(100) not null,
    nascimento date not null,
    stack varchar(32)[] null--,
--    search_content_pt  tsvector
--        GENERATED ALWAYS AS (to_tsvector('portuguese', nome || ' ' || apelido || coalesce(array_to_string(stack, ' ', ''), '')))
--       STORED
);

-- CREATE unique index ux_uid on pessoas (uid);
--CREATE INDEX terms ON pessoas USING GIN (search_content_pt);

--
--
-- INSERT INTO pessoas (apelido, uid, nome, nascimento, stack)
-- VALUES
--     ('john', 'fce87cb2-eb9c-4c14-b8e0-8e449f4cde93', 'John Doe', '1985-03-21', ARRAY['Java', 'Python']),
--     ('jane', '77b33c67-7311-4a32-bf72-3c05c619308c', 'Jane Smith', '1991-07-05', ARRAY['C#', 'JavaScript', 'React']),
--     ('mike', '0e5de8e0-11a7-4ceb-86c1-0d8d138e8f1e', 'Mike Johnson', '1980-12-15', NULL),
--     ('sarah', '23ceeb70-64d0-48c1-aa26-8a098f67a8e3', 'Sarah Lee', '1995-02-28', ARRAY['Python', 'Django']),
--     ('adam', '6640aa30-6188-46f5-bace-7bb279c3d23a', 'Adam Brown', '1994-10-11', ARRAY['C++', 'React', 'Node.js']),
--     ('mary', '2285f209-f6e4-4d71-867d-9b5163f805d4', 'Mary Taylor', '1987-06-01', NULL),
--     ('david', '6b3a51f2-4dc9-4c5b-876b-a86ec29a58d6', 'David Kim', '1990-11-23', ARRAY['Java', 'Scala']),
--     ('jessica', '7dd24d31-bdbe-4800-a920-7f1275af93e7', 'Jessica Davis', '1982-08-19', ARRAY['Python']),
--     ('hannah', '41cfc3aa-c7f3-41cf-b78c-9d3346e10b86', 'Hannah White', '1998-04-05', NULL),
--     ('brandon', '606c84ed-bae0-4940-a765-2eb417c31d62', 'Brandon Clark', '1993-01-17', ARRAY['PHP', 'Laravel']),
--     ('oliver', '2ddf1cb9-71a9-4539-aa78-a952a2b74374', 'Oliver Taylor', '1994-09-09', ARRAY['Java', 'JavaScript']),
--     ('joseph', 'cbb37c90-9957-4fa2-b102-b10eed614d29', 'Joseph Reyes', '1989-05-02', ARRAY['Python', 'Django']),
--     ('emily', '70915fd3-1ccd-4e83-b38d-569a327c7975', 'Emily Green', '1984-12-18', ARRAY['JavaScript', 'React']),
--     ('nathan', '312d7959-06a5-4067-bf61-d28ba20de9d2', 'Nathan Hernandez', '1992-07-23', ARRAY['Java', 'Spring']),
--     ('julia', 'b1f35f90-98a4-4ea3-bfce-f9cac22d55ec', 'Julia Kim', '1997-03-12', ARRAY['C++', 'Python']),
--     ('ariel', 'd984c7c4-efc1-47f5-ad1b-a4eb9a09a79d', 'Ariel Brown', '1991-11-29', ARRAY['PHP', 'Laravel', 'Vue.js']),
--     ('sebastian', '7f5621aa-d5d3-4465-9947-78fb1504243d', 'Sebastian Chen', '1996-08-08', ARRAY['JavaScript', 'React'])
-- ;
