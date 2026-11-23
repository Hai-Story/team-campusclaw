UPDATE grade_assignments SET subject=CASE subject WHEN 'science' THEN 'physics' WHEN 'morality' THEN 'politics' END WHERE subject IN ('science','morality');
